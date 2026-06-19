package l4

import (
	"fmt"
	"log/slog"
	"net"
	"time"
)

func (lb *LoadBalancer) HealthCheck() {
	var up int
	rt := lb.Runtime.Load()
	for idx := range rt.BackendPool.Backends {

		backend := &rt.BackendPool.Backends[idx]

		conn, err := net.DialTimeout("tcp", *backend.Address.Load(), 2*time.Second)
		if err != nil {
			failures := backend.ConsecutiveFailures.Add(1)
			backend.ConsecutiveSuccess.Store(0)

			if failures >= int32(rt.Config.HealthCheck.FailureThreshold) {
				backend.Up.Store(false)
			}

			continue
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			tcp.SetNoDelay(true)
		}
		conn.Close()
		success := backend.ConsecutiveSuccess.Add(1)
		backend.ConsecutiveFailures.Store(0)

		if success >= int32(rt.Config.HealthCheck.SuccessThreshold) {
			backend.Up.Store(true)
		}
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

	slog.Info("resources up", attrs...)
}
