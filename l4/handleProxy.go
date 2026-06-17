package l4

import (
	"lb-go/config"
	"log/slog"
	"net"

	"github.com/pires/go-proxyproto"
)

type handleProxyProps struct {
	backendConn net.Conn
	clientConn  net.Conn
	rt          *config.Runtime
	closeBoth   func()
}

func (lb *LoadBalancer) handleProxy(props *handleProxyProps) bool {

	if props.rt.Config.ProxyProtocol.Enabled {
		header := proxyproto.HeaderProxyFromAddrs(
			byte(*props.rt.Config.ProxyProtocol.Version),
			props.clientConn.RemoteAddr(),
			props.clientConn.LocalAddr(),
		)
		if _, err := header.WriteTo(props.backendConn); err != nil {
			slog.Error("failed to write proxy protocol header", "version", *props.rt.Config.ProxyProtocol.Version, "err", err)
			props.closeBoth()
			return false
		}

	}
	return true
}
