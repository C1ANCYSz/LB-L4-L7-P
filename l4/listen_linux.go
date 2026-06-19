package l4

import (
	"context"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/netutil"
	"golang.org/x/sys/unix"
)

func (lb *LoadBalancer) ListenReusePort(addr string) error {
	numCPU := runtime.NumCPU()
	slog.Info("Starting SO_REUSEPORT listener", slog.Int("listeners", numCPU))

	var wg sync.WaitGroup
	errChan := make(chan error, numCPU)

	// Create a context that we can cancel if one listener fails to start
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := range numCPU {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			lc := net.ListenConfig{
				Control: func(network, address string, c syscall.RawConn) error {
					return c.Control(func(fd uintptr) {
						err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
						if err != nil {
							slog.Warn("failed to set SO_REUSEPORT", slog.Any("err", err))
						}
					})
				},
			}

			listener, err := lc.Listen(ctx, "tcp", addr)
			if err != nil {
				errChan <- err
				cancel()
				return
			}
			defer listener.Close()

			rt := lb.Runtime.Load()
			limitedListener := netutil.LimitListener(listener, int(rt.Config.MaxConn))

			for {
				clientConn, err := limitedListener.Accept()
				if err != nil {
					select {
					case <-lb.Quit:
						return
					case <-ctx.Done():
						return
					default:
						slog.Error("accept error in reuseport listener", slog.Int("listener_id", id), slog.Any("err", err))
						continue
					}
				}

				clientIP := GetClientIP(clientConn.RemoteAddr())

				if rt.Config.TcpKeepAlive != nil {
					lb.ConfigureKeepAlive(clientConn, rt)
				}

				lb.ConnWG.Go(func() {
					lb.HandleConn(clientConn, clientIP)
				})
			}
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-errChan:
		return err
	default:
	}

	wg.Wait()
	return nil
}
