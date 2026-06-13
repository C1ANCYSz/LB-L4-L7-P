package main

import (
	"bytes"
	"io"
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

	go func() {
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

	servers := make([]Server, 1)
	servers[0].url = backend.Addr().String()
	servers[0].up.Store(true)

	lb := &LoadBalancer{
		logger:      logger,
		servers:     servers,
		balanceMode: LeastConn,
	}
	clientSide, lbSide := net.Pipe()
	defer clientSide.Close()
	t.Logf("client connected, handing to handleConn")

	go lb.handleConn(lbSide)

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
}
