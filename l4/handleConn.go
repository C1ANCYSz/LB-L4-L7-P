package l4

import (
	"lb-go/config"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func (lb *LoadBalancer) HandleConn(clientConn net.Conn, clientIP string) {

	rt := lb.Runtime.Load()

	var closeOnce sync.Once

	var copyWG sync.WaitGroup

	buf1 := bufPool.Get().(*[]byte)
	buf2 := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf1)
	defer bufPool.Put(buf2)

	backend := handleBalanceMode(rt, clientIP)
	if backend == nil {
		slog.Error("no upstream servers available")
		clientConn.Close()
		return
	}

	if rt.Config.TcpKeepAlive != nil {
		lb.ConfigureKeepAlive(clientConn, rt)
	}

	start := time.Now()
	dialStart := time.Now()

	newConnVal := backend.Connections.Add(1)

	if newConnVal > int64(backend.MaxConn.Load()) {
		backend.Connections.Add(-1)
		if tcp, ok := unwrapConn(clientConn).(*net.TCPConn); ok {
			tcp.SetLinger(0)
		}
		clientConn.Close()
		return
	}
	defer backend.Connections.Add(-1)

	backendConn, err := dialer.Dial("tcp", *backend.Address.Load())
	if err != nil {
		slog.Error("dial error", slog.Any("err", err))
		clientConn.Close()
		return
	}
	if rt.Config.TcpKeepAlive != nil {
		lb.ConfigureKeepAlive(backendConn, rt)
	}

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

	var idleTimeout *time.Duration
	if rt.Config.IdleTimeoutMs != nil {
		d := time.Duration(*rt.Config.IdleTimeoutMs) * time.Millisecond
		idleTimeout = &d
	}

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
		n, err := copyWithIdleTimeout(backendConn, clientConn, *buf1, idleTimeout)
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
		n, err := copyWithIdleTimeout(clientConn, backendConn, *buf2, idleTimeout)
		recvBytes.Add(n)

		recvErrKind := classifyConnError(err)
		if recvErrKind != ErrKindNone && recvErrKind != ErrKindBenign && recvErrKind != ErrKindCancelled {

			lb.handleConnError("backend → client", err, recvErrKind, backend)
			closeBoth()
			return
		}
		closeWrite(clientConn)
		closeBoth()

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
	if tcp, ok := unwrapConn(conn).(*net.TCPConn); ok {
		tcp.SetNoDelay(true)
	}
}

func closeWrite(conn net.Conn) {
	if tcp, ok := unwrapConn(conn).(*net.TCPConn); ok {
		tcp.CloseWrite()
	}
}

func copyWithIdleTimeout(dst net.Conn, src net.Conn, buf []byte, idle *time.Duration) (int64, error) {
	var total int64
	var lastUpdate time.Time

	// Set initial read deadline
	if idle != nil {
		src.SetReadDeadline(time.Now().Add(*idle))
		lastUpdate = time.Now()
	}

	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			if idle != nil && time.Since(lastUpdate) > 1*time.Second {
				src.SetReadDeadline(time.Now().Add(*idle))
				lastUpdate = time.Now()
			}
			nw, werr := dst.Write(buf[:nr])
			total += int64(nw)
			if werr != nil {
				return total, werr
			}
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return total, err
		}

		if err != nil {
			return total, err
		}
	}
}

func (lb *LoadBalancer) ConfigureKeepAlive(conn net.Conn, rt *config.Runtime) {

	tcp, ok := unwrapConn(conn).(*net.TCPConn)
	if !ok {
		return
	}
	err := tcp.SetKeepAliveConfig(
		net.KeepAliveConfig{
			Enable:   true,
			Idle:     time.Duration(rt.Config.TcpKeepAlive.IdleMs) * time.Millisecond,
			Interval: time.Duration(rt.Config.TcpKeepAlive.IntervalMs) * time.Millisecond,
			Count:    rt.Config.TcpKeepAlive.Count,
		},
	)
	if err != nil {
		slog.Warn("failed to configure keepalive",
			slog.Any("err", err))
	}

}
