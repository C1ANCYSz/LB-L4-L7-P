package main

import (
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	ticker := time.NewTicker(time.Second * 5)

	serversUrls, balanceMode, lbLevel, maxConn := LoadConfig()

	resolvedURLs := make([]string, len(serversUrls))
	for i, url := range serversUrls {
		resolved, err := resolveHost(url)
		if err != nil {
			log.Fatalf("failed to resolve %s: %v", url, err)
		}
		resolvedURLs[i] = resolved
		log.Printf("resolved %s → %s", url, resolved)
	}

	servers := make([]Server, 0, len(serversUrls))
	for _, url := range resolvedURLs {
		servers = append(servers, Server{url: url})
	}

	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	lb := &LoadBalancer{
		quit:        make(chan struct{}),
		logger:      logger,
		servers:     servers,
		balanceMode: balanceMode,
		level:       lbLevel,
		maxConn:     maxConn,
	}
	lb.startReResolver(serversUrls, time.Second*1)
	lb.pingServers()
	go func() {
		if err := lb.Listen(":8080"); err != nil {
			log.Fatal(err)
		}
	}()
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
