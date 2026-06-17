package main

import (
	config "lb-go/config"
	"lb-go/l4"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"
)

func main() {

	InitLogger()

	quit := GracefulShutdownChan()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	lb := &l4.LoadBalancer{
		Quit:     quit,
		Listener: net.Listener(nil),
		ConnWG:   sync.WaitGroup{},
	}

	configManager := config.NewConfigManager(cfg, func(cfg *config.Config) {
		lb.Reload(cfg)
		lb.ResolveAllBackends()
		lb.PingServers()
	})

	go configManager.Watch()

	runtime := config.NewRuntime(cfg)
	lb.Runtime.Store(runtime)

	pingTicker := time.NewTicker(time.Duration(configManager.Get().PingIntervalMs) * time.Millisecond)
	defer pingTicker.Stop()

	//for keeping track of goroutines
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	lb.ResolveAllBackends()
	lb.StartDNSResolver(time.Duration(configManager.Get().DNSRefreshIntervalMs) * time.Millisecond)
	lb.PingServers()

	go func() {
		if err := lb.Listen(":8080"); err != nil {
			log.Fatal(err)
		}
	}()

	for {
		select {
		case <-quit:
			{
				lb.Shutdown()
				return
			}

		case <-pingTicker.C:
			{
				lb.PingServers()

				newInterval := time.Duration(configManager.Get().PingIntervalMs) * time.Millisecond
				pingTicker.Reset(newInterval)
			}

		}

	}

}
