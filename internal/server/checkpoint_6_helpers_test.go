//go:build all || m6 || m6_2 || m7 || m7_2

// Shared helpers for the server checkpoints (6.2 and 7.2).

package server_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/server"
)

type running struct {
	url    string
	cancel context.CancelFunc
	done   chan error
}

func startServer(t *testing.T) *running {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{url: "http://" + ln.Addr().String(), cancel: cancel, done: make(chan error, 1)}
	go func() {
		r.done <- server.Run(ctx, ln, server.Config{Feed: "fake", FakeInterval: 20 * time.Millisecond})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(5 * time.Second):
			t.Error("server.Run did not return within 5s of cancelling its context")
		}
	})
	return r
}
