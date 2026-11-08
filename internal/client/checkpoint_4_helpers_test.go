//go:build all || m4 || m4_9 || m6 || m6_1

// Shared helpers for the client session checkpoints (4.9 and 6.1).

package client_test

import (
	"context"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/client"
	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/testkit"
)

type nopUpstream struct{}

func (nopUpstream) Subscribe([]string) error   { return nil }
func (nopUpstream) Unsubscribe([]string) error { return nil }

type env struct {
	hub      *hub.Hub
	srv      *httptest.Server
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func setup(t *testing.T, hcfg hub.Config, ccfg client.Config) *env {
	t.Helper()
	if hcfg.Upstream == nil {
		hcfg.Upstream = nopUpstream{}
	}
	h := hub.New(hcfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Run(ctx)
	}()
	handler := client.NewHandler(h, ccfg)
	if handler == nil {
		t.Fatal("client.NewHandler returned nil")
	}
	srv := httptest.NewServer(handler)
	e := &env{hub: h, srv: srv, cancel: cancel, done: done}
	t.Cleanup(func() {
		srv.CloseClientConnections()
		srv.Close()
		e.stopHub(t)
	})
	return e
}

func (e *env) stopHub(t *testing.T) {
	t.Helper()
	e.stopOnce.Do(func() {
		e.cancel()
		select {
		case <-e.done:
		case <-time.After(2 * time.Second):
			t.Error("Hub.Run did not return within 2s of cancelling its context")
		}
	})
}

func (e *env) waitClients(t *testing.T, n int) {
	t.Helper()
	testkit.Eventually(t, 3*time.Second, func() bool { return e.hub.Stats().Clients == n },
		"waited for the hub to have %d registered client(s); it has %v", n, lazyClients{e.hub})
}

func (e *env) waitSubscribed(t *testing.T, sym string, n int) {
	t.Helper()
	testkit.Eventually(t, 3*time.Second, func() bool { return e.hub.Stats().Subscriptions[sym] == n },
		"waited for %s to have %d subscriber(s) in the hub", sym, n)
}

type lazyClients struct{ h *hub.Hub }

func (l lazyClients) String() string { return strconv.Itoa(l.h.Stats().Clients) }

var ts0 = time.Date(2026, 11, 6, 18, 30, 0, 0, time.UTC)

func q(sym string, bid float64) market.Quote {
	return market.Quote{Symbol: sym, BidPrice: bid, BidSize: 1, AskPrice: bid + 0.01, AskSize: 1, Timestamp: ts0}
}
