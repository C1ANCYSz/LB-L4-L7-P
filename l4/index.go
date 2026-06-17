package l4

import (
	"lb-go/config"
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
	Quit     chan os.Signal
	Listener net.Listener
	ConnWG   sync.WaitGroup
	Logger   *slog.Logger

	Runtime atomic.Pointer[config.Runtime]
}

var dialer = &net.Dialer{
	Timeout:   time.Second,
	KeepAlive: 30 * time.Second,
}

func (lb *LoadBalancer) Reload(cfg *config.Config) {
	newRt := config.NewRuntime(cfg)
	lb.Runtime.Store(newRt)
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
