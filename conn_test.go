package main

import (
	"bytes"
	"io"
	config "lb-go/config"
	"lb-go/l4"
	"lb-go/resources"
	"log/slog"
	"net"
	"os"
	"testing"
)

func TestHandleConn(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	t.Logf("backend listening on %s", backend.Addr())

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		t.Logf("backend accepted connection from %s", conn.RemoteAddr())
		io.Copy(conn, conn)
		t.Logf("backend done echoing")
	}()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	servers := make([]resources.Backend, 1)
	addr := backend.Addr().String()
	servers[0].OriginalAddress = addr
	servers[0].Address.Store(&addr)
	servers[0].Up.Store(true)

	rt := &config.Runtime{
		Config: &config.Config{
			BalanceMode:   config.LeastConn,
			IdleTimeoutMs: 200,
		},
		BackendPool: &resources.BackendPool{
			Backends: servers,
		},
	}
	lb := &l4.LoadBalancer{
		Logger: logger,
	}
	lb.Runtime.Store(rt)
	clientSide, lbSide := tcpPipe(t)
	t.Logf("client connected, handing to handleConn")

	go lb.HandleConn(lbSide)

	msg := []byte("hello backend")
	t.Logf("client sending: %q", msg)
	_, err = clientSide.Write(msg)
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(msg))
	_, err = io.ReadFull(clientSide, buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("client received: %q", buf)

	if !bytes.Equal(buf, msg) {
		t.Fatalf("expected %q got %q", msg, buf)
	}
	t.Log("✓ data round-tripped through LB")

	clientSide.Close()
	<-done
}
func tcpPipe(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		done <- c
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return client, <-done
}
