package l4

import (
	"lb-go/config"
	"lb-go/infra"
	"net"
	"os"
	"os/signal"
	"reflect"
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
	cfg.LogConfig()

}

func (lb *LoadBalancer) Shutdown() {
	signal.Stop(lb.Quit)
	close(lb.Quit)
	lb.Listener.Close()
	lb.ConnWG.Wait()
}

func unwrapConn(conn net.Conn) net.Conn {
	rawConn := conn
	for {
		val := reflect.ValueOf(rawConn)
		if val.Kind() == reflect.Pointer {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			break
		}
		field := val.FieldByName("Conn")
		if !field.IsValid() {
			break
		}
		if nextConn, ok := field.Interface().(net.Conn); ok {
			rawConn = nextConn
		} else {
			break
		}
	}
	return rawConn
}
