package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	serversUrls, balanceMode := LoadConfig()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	ticker := time.NewTicker(time.Second * 20)
	servers := make([]Server, 0, len(serversUrls))
	for _, url := range serversUrls {
		servers = append(servers, Server{
			url:  url,
			pool: make(chan net.Conn, 10),
			// up defaults to false (zero value), connections defaults to 0
		})
	}

	lb := &LoadBalancer{
		quit:        make(chan struct{}),
		logger:      logger,
		servers:     servers,
		balanceMode: balanceMode,
	}

	go func() {
		if err := lb.Listen(":8080"); err != nil {
			log.Fatal(err)
		}
	}()
	lb.pingServers()
	for {
		select {
		case <-quit:
			ticker.Stop()
			lb.Shutdown()
			return

		case <-ticker.C:
			lb.pingServers()

		}

	}

}
