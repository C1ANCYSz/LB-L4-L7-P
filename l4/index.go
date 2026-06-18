package l4

import (
	"lb-go/config"
	"lb-go/infra"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"
)

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

type LoadBalancer struct {
	Quit        chan os.Signal
	Listener    net.Listener
	ConnWG      sync.WaitGroup
	RateLimiter atomic.Pointer[infra.RateLimiter]
	Runtime     atomic.Pointer[config.Runtime]
}

var dialer = &net.Dialer{
	Timeout:   time.Second,
	KeepAlive: 30 * time.Second,
}

func (lb *LoadBalancer) Reload(cfg *config.Config) {

	lb.Runtime.Store(config.NewRuntime(cfg))
	if rl := lb.RateLimiter.Load(); rl != nil {
		rl.Update(cfg.RateLimit)
	}
	slog.Info(
		"configuration reloaded",
		"balance_mode", cfg.BalanceMode,
		"rate_limit", cfg.RateLimit,
	)

}

func (lb *LoadBalancer) Shutdown() {
	signal.Stop(lb.Quit)
	close(lb.Quit)
	lb.Listener.Close()
	lb.ConnWG.Wait()
}

// TODO: implement SO_REUSEPORT for Linux production deployments
// Use build tags: //go:build linux
// Spawn runtime.NumCPU() listeners for parallel accept() loops
func (lb *LoadBalancer) ListenReusePort(addr string) error {
	return nil
}
