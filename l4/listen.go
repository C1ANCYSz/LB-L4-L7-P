package l4

import (
	"log/slog"
	"net"

	"golang.org/x/net/netutil"
)

func (lb *LoadBalancer) Listen(addr string) error {
	//init listener on tcp
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	rt := lb.Runtime.Load()

	//limit listener to 10K concurrent listeners to avoid go routines leaks
	lb.Listener = netutil.LimitListener(listener, int(rt.Config.MaxConn))
	defer lb.Listener.Close()

	for {
		conn, err := lb.Listener.Accept()
		if err != nil {
			select {
			case <-lb.Quit:

				return nil
			default:
				lb.Logger.Error("accept error ", slog.Any("err", err))
				continue
			}
		}
		lb.ConnWG.Go(func() {

			lb.HandleConn(conn)
		})

	}
}
