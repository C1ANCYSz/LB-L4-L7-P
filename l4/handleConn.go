package l4

import (
	"lb-go/config"
	"lb-go/resources"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func (lb *LoadBalancer) HandleConn(clientConn net.Conn) {

	buf1 := bufPool.Get().(*[]byte)
	buf2 := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf1)
	defer bufPool.Put(buf2)

	var backend *resources.Backend
	ip := getClientIP(clientConn.RemoteAddr())

	wg := sync.WaitGroup{}
	rt := lb.Runtime.Load()

	start := time.Now()
	switch rt.Config.BalanceMode {
	case config.IpHash:
		backend = rt.BackendPool.GetIPHashServer(ip)
	default:
		backend = rt.BackendPool.GetLeastConnServer()
	}

	if backend == nil {
		lb.Logger.Error("no upstream servers available")
		clientConn.Close()
		return
	}
	dialStart := time.Now()
	backendConn, err := dialer.Dial("tcp", *backend.Address.Load())
	if err != nil {
		lb.Logger.Error("dial error", slog.Any("err", err))
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

	if tcp, ok := backendConn.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}
	if tcp, ok := clientConn.(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}

	dialTook := time.Since(dialStart)

	var sentBytes, recvBytes atomic.Int64
	connTimeout := time.Duration(rt.Config.IdleTimeoutMs) * time.Millisecond
	wg.Go(func() {
		n, err := copyWithIdleTimeout(backendConn, clientConn, *buf1, connTimeout)
		sentBytes.Add(n)

		sendErrKind := classifyConnError(err)
		if sendErrKind != ErrKindNone && sendErrKind != ErrKindBenign && sendErrKind != ErrKindCancelled {
			lb.handleConnError("client → backend", err, sendErrKind, backend)
			closeBoth()
			return
		}

		if tcp, ok := backendConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}

	})

	wg.Go(func() {
		n, err := copyWithIdleTimeout(clientConn, backendConn, *buf2, connTimeout)
		recvBytes.Add(n)
		recvErrKind := classifyConnError(err)
		if recvErrKind != ErrKindNone && recvErrKind != ErrKindBenign && recvErrKind != ErrKindCancelled {
			lb.handleConnError("backend → client", err, recvErrKind, backend)
			closeBoth()
			return
		}
		if tcp, ok := clientConn.(*net.TCPConn); ok {
			tcp.CloseWrite()

		}
	})

	wg.Wait()

	if rt.Config.Debug {
		lb.Logger.Info("served",
			slog.String("client", ip),
			slog.String("server", *backend.Address.Load()),
			slog.Duration("dial_took", dialTook),
			slog.Duration("total", time.Since(start)),
			slog.Int64("sent_bytes", sentBytes.Load()),
			slog.Int64("recv_bytes", recvBytes.Load()),
			slog.Int64("server_connections", backend.Connections.Load()),
		)
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
