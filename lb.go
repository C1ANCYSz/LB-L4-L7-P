package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/netutil"
)

type LoadBalancer struct {
	listener    net.Listener
	quit        chan struct{}
	wg          sync.WaitGroup
	logger      *slog.Logger
	servers     []Server
	balanceMode BalanceMode
}

type Server struct {
	url         string
	up          atomic.Bool
	connections int64
	pool        chan net.Conn
}

func (s *Server) getConn() (net.Conn, error) {
	select {
	case conn := <-s.pool:
		log.Printf("REUSE pool size=%d cap=%d", len(s.pool), cap(s.pool))
		return conn, nil
	default:
		log.Printf("DIAL pool size=%d cap=%d", len(s.pool), cap(s.pool))
		return net.DialTimeout("tcp", s.url, 5*time.Second)
	}
}

func (s *Server) returnConn(conn net.Conn) {
	select {
	case s.pool <- conn:
		log.Printf("RETURN pool size=%d cap=%d", len(s.pool), cap(s.pool))
	default:
		log.Printf("DISCARD pool full size=%d cap=%d", len(s.pool), cap(s.pool))
		conn.Close()
	}
}
func isBenignConnError(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "forcibly closed by the remote host") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "use of closed network connection")
}
func (lb *LoadBalancer) Listen(addr string) error {
	//init listener on tcp
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	//limit listener to 10K concurrent listeners to avoid go routines leaks
	lb.listener = netutil.LimitListener(listener, 100)
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
		resp, err := http.Get("http://" + server.url)
		if err != nil {
			lb.logger.Error("ping error", slog.Any("err", err))
			lb.servers[idx].up.Store(false)
			continue
		} else {
			lb.servers[idx].up.Store(true)

		}
		up++
		resp.Body.Close()

	}

	msg := fmt.Sprintf("%d/%d are up", up, len(lb.servers))

	lb.logger.Info(msg)

}

func (lb *LoadBalancer) getConnServer() *Server {

	var server *Server

	switch lb.balanceMode {
	case RoundRobin:
		// TODO

	case LeastConn:
		for i := range lb.servers {
			s := &lb.servers[i]
			if !s.up.Load() {
				continue
			}
			if server == nil || atomic.LoadInt64(&s.connections) < atomic.LoadInt64(&server.connections) {
				server = s
			}
		}

	case IpHash:
		// TODO
	}

	return server
}
func (lb *LoadBalancer) handleConn(clientConn net.Conn) {
	wg := sync.WaitGroup{}
	start := time.Now()

	server := lb.getConnServer()
	if server == nil {
		lb.logger.Error("no upstream servers available")
		clientConn.Close()
		return
	}
	atomic.AddInt64(&server.connections, 1)
	defer atomic.AddInt64(&server.connections, -1)

	dialStart := time.Now()
	backendConn, err := net.DialTimeout("tcp", server.url, 5*time.Second)
	if err != nil {
		lb.logger.Error("dial error", slog.Any("err", err))
		clientConn.Close()
		return
	}
	defer backendConn.Close()
	defer clientConn.Close()

	dialTook := time.Since(dialStart)

	var sentBytes, recvBytes int64
	var sendErr, recvErr error

	wg.Go(func() {
		n, err := io.Copy(backendConn, clientConn)
		sentBytes = n
		sendErr = err
		if tcp, ok := backendConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	})

	wg.Go(func() {
		n, err := io.Copy(clientConn, backendConn)
		recvBytes = n
		recvErr = err
		if tcp, ok := clientConn.(*net.TCPConn); ok {
			tcp.CloseWrite()
		}
	})

	wg.Wait()

	// Log any non-benign errors individually
	if !isBenignConnError(sendErr) {
		lb.logger.Error("client → backend", slog.Any("err", sendErr))
	}
	if !isBenignConnError(recvErr) {
		lb.logger.Error("backend → client", slog.Any("err", recvErr))
	}

	lb.logger.Info("served",
		slog.String("client", clientConn.RemoteAddr().String()),
		slog.String("server", server.url),
		slog.Duration("dial_took", dialTook),
		slog.Duration("total", time.Since(start)),
		slog.Int64("sent_bytes", sentBytes),
		slog.Int64("recv_bytes", recvBytes),
	)
}

func (lb *LoadBalancer) Shutdown() {
	close(lb.quit)
	lb.listener.Close()
}
