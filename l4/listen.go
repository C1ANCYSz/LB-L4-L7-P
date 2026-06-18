package l4

import (
	"log/slog"
	"net"

	"golang.org/x/net/netutil"
)

func (lb *LoadBalancer) Listen(addr string) error {
	//init listener on tcp

	rt := lb.Runtime.Load()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	//limit listener to 10K concurrent listeners to avoid go routines leaks
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
		if rt.Config.TcpKeepAlive != nil {
			lb.ConfigureKeepAlive(clientConn)
		}
		lb.ConnWG.Go(func() {

			lb.HandleConn(clientConn)
		})

	}
}
