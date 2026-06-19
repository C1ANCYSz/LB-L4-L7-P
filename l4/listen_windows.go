package l4

import (
	"log/slog"
)

func (lb *LoadBalancer) ListenReusePort(addr string) error {
	slog.Info("SO_REUSEPORT is not supported on this OS, falling back to standard Listen")
	return lb.Listen(addr)
}
