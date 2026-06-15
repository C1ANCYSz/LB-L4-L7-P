package l4

import (
	"fmt"
	"log/slog"
	"net"
	"time"
)

func (lb *LoadBalancer) PingServers() {
	var up int
	rt := lb.Runtime.Load()
	for idx := range rt.BackendPool.Backends {
		backend := &rt.BackendPool.Backends[idx]
		pingEndpoint := *backend.Address.Load() + backend.PingEndpoint
		conn, err := net.DialTimeout("tcp", pingEndpoint, 2*time.Second)
		if err != nil {
			backend.Up.Store(false)
			continue
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			tcp.SetNoDelay(true)
		}
		conn.Close()
		backend.Up.Store(true)
		up++
	}

	attrs := []any{
		slog.Int("up", up),
		slog.Int("total", len(rt.BackendPool.Backends)),
	}

	for i := range rt.BackendPool.Backends {
		if url := rt.BackendPool.Backends[i].Address.Load(); url != nil {
			attrs = append(attrs,
				slog.String(fmt.Sprintf("backend_%d", i+1), *url),
			)
		}
	}

	lb.Logger.Info("resources up", attrs...)
}
