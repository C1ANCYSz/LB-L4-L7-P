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

	logger := CreateLogger()

	quit := GracefulShutdownChan()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	configManager := config.NewConfigManager(cfg, logger)

	go configManager.Watch()

	pingServersTicker := time.NewTicker(time.Duration(configManager.Get().PingIntervalMs) * time.Millisecond)

	//for keeping track of goroutines
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	runtime := config.NewRuntime(cfg)
	lb := &l4.LoadBalancer{
		Quit:     quit,
		Listener: net.Listener(nil),
		Wg:       sync.WaitGroup{},
		Logger:   logger,
	}
	lb.Runtime.Store(runtime)
	configManager.OnReload = func(cfg *config.Config) {
		lb.Reload(cfg)
		lb.ResolveAllBackends()
		lb.PingServers()

	}
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
			pingServersTicker.Stop()
			{
				lb.Shutdown()
				return
			}

		case <-pingServersTicker.C:
			{
				lb.PingServers()

				newInterval := time.Duration(configManager.Get().PingIntervalMs) * time.Millisecond
				pingServersTicker.Reset(newInterval)
			}

		}

	}

}
