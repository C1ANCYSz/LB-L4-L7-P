package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/netutil"
)

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 256*1024)
		return &buf
	},
}

type LoadBalancer struct {
	listener    net.Listener
	quit        chan struct{}
	wg          sync.WaitGroup
	logger      *slog.Logger
	servers     []Server
	balanceMode BalanceMode
	Debug       bool
	level       LBLevel
	maxConn     int
}

var dialer = &net.Dialer{
	Timeout:   time.Second,
	KeepAlive: 30 * time.Second,
}

func (lb *LoadBalancer) Listen(addr string) error {
	//init listener on tcp
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	//limit listener to 10K concurrent listeners to avoid go routines leaks
	lb.listener = netutil.LimitListener(listener, int(lb.maxConn))
	defer lb.listener.Close()

	for {
		conn, err := lb.listener.Accept()
		if err != nil {
			select {
			case <-lb.quit:
				lb.wg.Wait()
				return nil
			default:
				lb.logger.Error("accept error ", slog.Any("err", err))
				continue
			}
		}
		lb.wg.Go(func() {

			lb.handleConn(conn)
		})

	}
}

func (lb *LoadBalancer) pingServers() {
	var up int
	for idx := range lb.servers {
		server := &lb.servers[idx]
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://" + server.url + "/health")
		if err != nil {
			lb.logger.Error("ping error", slog.Any("err", err))
			server.up.Store(false)
			continue
		}
		resp.Body.Close()
		server.up.Store(true)
		up++
	}
	lb.logger.Info(fmt.Sprintf("%d/%d are up", up, len(lb.servers)))

}
func (lb *LoadBalancer) startReResolver(originalURLs []string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			lb.reResolveServers(originalURLs)
		}
	}()
}
func (lb *LoadBalancer) reResolveServers(originalURLs []string) {
	for i, original := range originalURLs {
		resolved, err := resolveHost(original)
		if err != nil {
			continue
		}
		lb.servers[i].url = resolved
	}
}

// call every 30s in a goroutine alongside pingServers

func (lb *LoadBalancer) getIPHashServer(ip string) *Server {
	h := fnv.New32a()
	h.Write([]byte(ip))
	idx := int(h.Sum32()) % len(lb.servers)

	// Try the hashed server first, fall back to least-conn if it's down
	if lb.servers[idx].up.Load() {
		return &lb.servers[idx]
	}
	return lb.getLeastConnServer()
}
func (lb *LoadBalancer) getLeastConnServer() *Server {

	var server *Server
	for i := range lb.servers {
		s := &lb.servers[i]
		if !s.up.Load() {
			continue
		}
		if server == nil || s.connections.Load() < server.connections.Load() {
			server = s
		}
	}

	return server
}
func (lb *LoadBalancer) handleConn(clientConn net.Conn) {

	buf1 := bufPool.Get().(*[]byte)
	buf2 := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf1)
	defer bufPool.Put(buf2)

	var server *Server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ip := getClientIP(clientConn.RemoteAddr())

	wg := sync.WaitGroup{}

	start := time.Now()
	switch lb.balanceMode {
	case IpHash:
		server = lb.getIPHashServer(ip)
	default:
		server = lb.getLeastConnServer()
	}

	if server == nil {
		lb.logger.Error("no upstream servers available")
		clientConn.Close()
		return
	}
	server.connections.Add(1)
	defer server.connections.Add(-1)

	dialStart := time.Now()
	backendConn, err := dialer.DialContext(ctx, "tcp", server.url)
	if err != nil {
		lb.logger.Error("dial error", slog.Any("err", err))
		clientConn.Close()
		return
	}

	go func() {
		<-ctx.Done()
		backendConn.Close()
		clientConn.Close()
	}()

	dialTook := time.Since(dialStart)

	var sentBytes, recvBytes int64

	wg.Go(func() {
		n, err := io.CopyBuffer(backendConn, clientConn, *buf1)
		sentBytes = n

		sendErrKind := classifyConnError(err)
		if sendErrKind != ErrKindNone && sendErrKind != ErrKindBenign && sendErrKind != ErrKindCancelled {
			lb.handleConnError("client → backend", err, sendErrKind, server)
			cancel()
			return
		}

		if tcp, ok := backendConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	})
	wg.Go(func() {
		n, err := io.CopyBuffer(clientConn, backendConn, *buf2)
		recvBytes = n
		recvErrKind := classifyConnError(err)
		if recvErrKind != ErrKindNone && recvErrKind != ErrKindBenign && recvErrKind != ErrKindCancelled {
			lb.handleConnError("backend → client", err, recvErrKind, server)
			cancel()
			return
		}
		if tcp, ok := clientConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	})

	wg.Wait()

	if lb.Debug {
		lb.logger.Info("served",
			slog.String("client", ip),
			slog.String("server", server.url),
			slog.Duration("dial_took", dialTook),
			slog.Duration("total", time.Since(start)),
			slog.Int64("sent_bytes", sentBytes),
			slog.Int64("recv_bytes", recvBytes),
			slog.Int64("server_connections", server.connections.Load()),
		)
	}
}

func (lb *LoadBalancer) handleConnError(direction string, err error, kind ConnErrKind, server *Server) {
	switch kind {
	case ErrKindNone, ErrKindBenign, ErrKindCancelled:
		// expected, swallow
	case ErrKindTimeout:
		lb.logger.Warn("connection timed out",
			slog.String("direction", direction),
			slog.Any("err", err),
		)
	case ErrKindRefused:
		// backend is down — mark it
		server.up.Store(false)
		lb.logger.Error("backend refused connection",
			slog.String("direction", direction),
			slog.String("server", server.url),
			slog.Any("err", err),
		)
	case ErrKindUnreachable:
		server.up.Store(false)
		lb.logger.Error("backend unreachable",
			slog.String("direction", direction),
			slog.String("server", server.url),
			slog.Any("err", err),
		)
	case ErrKindExhausted:
		// this is a system-level emergency
		lb.logger.Error("RESOURCE EXHAUSTION — file descriptors or memory",
			slog.String("direction", direction),
			slog.Any("err", err),
		)
	default:
		// ErrKindUnknown — log everything, don't swallow
		lb.logger.Error("unclassified connection error",
			slog.String("direction", direction),
			slog.String("kind", kind.String()),
			slog.Any("err", err),
		)
	}
}

func (lb *LoadBalancer) Shutdown() {
	close(lb.quit)
	lb.listener.Close()
}

func getClientIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP.String()
	case *net.UDPAddr:
		return a.IP.String()
	}
	// Fallback for mock/pipe/unix domain socket addresses
	str := addr.String()
	if host, _, err := net.SplitHostPort(str); err == nil {
		return host
	}
	return str
}

