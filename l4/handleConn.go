package l4

import (
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func (lb *LoadBalancer) HandleConn(clientConn net.Conn) {
	rt := lb.Runtime.Load()
	var copyWG sync.WaitGroup
	buf1 := bufPool.Get().(*[]byte)
	buf2 := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf1)
	defer bufPool.Put(buf2)

	clientIP := getClientIP(clientConn.RemoteAddr())

	start := time.Now()

	backend := handleBalanceMode(rt, clientIP)
	if backend == nil {
		slog.Error("no upstream servers available")
		clientConn.Close()
		return
	}

	dialStart := time.Now()
	backendConn, err := dialer.Dial("tcp", *backend.Address.Load())
	if err != nil {
		slog.Error("dial error", slog.Any("err", err))
		clientConn.Close()
		return
	}
	backend.Connections.Add(1)
	defer backend.Connections.Add(-1)

	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			clientConn.Close()
			backendConn.Close()
		})
	}
	defer closeBoth()
	setNoDelay(backendConn)
	setNoDelay(clientConn)

	dialTook := time.Since(dialStart)

	var sentBytes, recvBytes atomic.Int64
	connTimeout := time.Duration(rt.Config.IdleTimeoutMs) * time.Millisecond

	ok := lb.handleProxy(&handleProxyProps{
		clientConn:  clientConn,
		backendConn: backendConn,
		rt:          rt,
		closeBoth:   closeBoth,
	})

	if !ok {
		return
	}

	copyWG.Go(func() {
		n, err := copyWithIdleTimeout(backendConn, clientConn, *buf1, connTimeout)
		sentBytes.Add(n)

		sendErrKind := classifyConnError(err)
		if sendErrKind != ErrKindNone && sendErrKind != ErrKindBenign && sendErrKind != ErrKindCancelled {
			lb.handleConnError("client → backend", err, sendErrKind, backend)
			closeBoth()
			return
		}

		closeWrite(backendConn)

	})

	copyWG.Go(func() {
		n, err := copyWithIdleTimeout(clientConn, backendConn, *buf2, connTimeout)
		recvBytes.Add(n)
		recvErrKind := classifyConnError(err)
		if recvErrKind != ErrKindNone && recvErrKind != ErrKindBenign && recvErrKind != ErrKindCancelled {
			lb.handleConnError("backend → client", err, recvErrKind, backend)
			closeBoth()
			return
		}
		closeWrite(clientConn)

	})

	copyWG.Wait()

	if rt.Config.Debug {
		slog.Info("served",
			slog.String("client", clientIP),
			slog.String("server", *backend.Address.Load()),
			slog.Duration("dial_took", dialTook),
			slog.Duration("total", time.Since(start)),
			slog.Int64("sent_bytes", sentBytes.Load()),
			slog.Int64("recv_bytes", recvBytes.Load()),
			slog.Int64("server_connections", backend.Connections.Load()),
		)
	}
}

func setNoDelay(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}
}

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.CloseWrite()
	}
}

func getClientIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP.String()
	case *net.UDPAddr:
		return a.IP.String()
	}
	// Fallback for mock/pipe/unix domain socket addresses
	str := addr.String()
	if host, _, err := net.SplitHostPort(str); err == nil {
		return host
	}
	return str
}
func copyWithIdleTimeout(dst net.Conn, src net.Conn, buf []byte, idle time.Duration) (int64, error) {
	var total int64
	for {
		src.SetReadDeadline(time.Now().Add(idle))
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			total += int64(nw)
			if werr != nil {
				return total, werr
			}
		}
		if err != nil {
			return total, err
		}
	}
}
