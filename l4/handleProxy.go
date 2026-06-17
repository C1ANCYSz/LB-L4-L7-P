package l4

import (
	"fmt"
	"lb-go/config"
	"log/slog"
	"net"

	"github.com/pires/go-proxyproto"
)

type handleProxyProps struct {
	backendConn    net.Conn
	clientConn     net.Conn
	rt             *config.Runtime
	backendAddress *string
	closeBoth      func()
}

func (lb *LoadBalancer) handleProxy(props *handleProxyProps) bool {
	clientIP := getClientIP(props.clientConn.RemoteAddr())
	clientPort := props.clientConn.RemoteAddr().(*net.TCPAddr).Port
	backendPort := props.backendConn.RemoteAddr().(*net.TCPAddr).Port

	if props.rt.Config.ProxyProtocol.Enabled {
		switch *props.rt.Config.ProxyProtocol.Version {
		case config.V1:
			{
				network := "TCP4"
				if net.ParseIP(clientIP).To4() == nil {
					network = "TCP6"
				}
				header := fmt.Sprintf("PROXY %s %s %s %d %d\r\n", network, clientIP, *props.backendAddress, clientPort, backendPort)
				if _, err := fmt.Fprint(props.backendConn, header); err != nil {
					slog.Error("failed to write proxy protocol header V1", slog.Any("err", err))
					props.closeBoth()
					return false
				}
			}
		case config.V2:
			{
				header := proxyproto.HeaderProxyFromAddrs(
					2,
					props.clientConn.RemoteAddr(),
					props.backendConn.LocalAddr(),
				)

				_, err := header.WriteTo(props.backendConn)
				if err != nil {
					slog.Error("failed to write proxy protocol header V2", slog.Any("err", err))
					props.closeBoth()
					return false

				}
			}
		}

	}
	return true
}
