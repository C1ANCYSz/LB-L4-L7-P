package l4

import (
	"log/slog"
	"net"

	"golang.org/x/net/netutil"
)

func (lb *LoadBalancer) Listen(addr string) error {

	rt := lb.Runtime.Load()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	lb.Listener = netutil.LimitListener(listener, int(rt.Config.MaxConn))
	defer lb.Listener.Close()

	for {

		clientConn, err := lb.Listener.Accept()

		if err != nil {
			select {
			case <-lb.Quit:

				return nil
			default:
				slog.Error("accept error ", slog.Any("err", err))
				continue
			}
		}

		clientIP := GetClientIP(clientConn.RemoteAddr())
		if !lb.RateLimiter.Load().Allow(clientIP) {

			if tcp, ok := unwrapConn(clientConn).(*net.TCPConn); ok {
				tcp.SetLinger(0)
			}
			clientConn.Close()
			continue
		}

		lb.ConnWG.Go(func() {
			lb.HandleConn(clientConn, clientIP)
		})

	}
}
func GetClientIP(addr net.Addr) string {
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
