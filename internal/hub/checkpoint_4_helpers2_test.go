//go:build all || m4 || m4_2 || m4_3 || m4_4 || m4_5 || m4_6 || m4_7

// Shared helpers for hub checkpoints 4.2 to 4.7.

package hub_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/testkit"
)

var ts0 = time.Date(2026, 11, 6, 18, 30, 0, 0, time.UTC)

func quote(sym string, bid float64) market.Quote {
	return market.Quote{Symbol: sym, BidPrice: bid, BidSize: 1, AskPrice: bid + 0.01, AskSize: 2, Timestamp: ts0}
}

func subscribe(t *testing.T, h *testHub, who string, c *hub.Client, symbols ...string) {
	t.Helper()
	var err error
	testkit.Within(t, 2*time.Second, fmt.Sprintf("Subscribe(%s, %v)", who, symbols), func() {
		err = h.Subscribe(c, symbols)
	})
	if err != nil {
		t.Fatalf("Subscribe(%s, %v) returned error %v", who, symbols, err)
	}
}

func unsubscribe(t *testing.T, h *testHub, who string, c *hub.Client, symbols ...string) {
	t.Helper()
	var err error
	testkit.Within(t, 2*time.Second, fmt.Sprintf("Unsubscribe(%s, %v)", who, symbols), func() {
		err = h.Unsubscribe(c, symbols)
	})
	if err != nil {
		t.Fatalf("Unsubscribe(%s, %v) returned error %v", who, symbols, err)
	}
}

func publishQuote(t *testing.T, h *testHub, q market.Quote) {
	t.Helper()
	testkit.Within(t, 2*time.Second, "PublishQuote", func() { h.PublishQuote(q) })
}

func expectQuote(t *testing.T, who string, c *hub.Client, sym string, bid float64) {
	t.Helper()
	m := next(t, who, c)
	if m["type"] != "quote" || m["symbol"] != sym || m["bid"] != bid {
		t.Fatalf("%s got %v, want the %s quote with bid %v", who, m, sym, bid)
	}
}

func trySubscribe(t *testing.T, h *testHub, c *hub.Client, symbols ...string) error {
	t.Helper()
	var err error
	testkit.Within(t, 2*time.Second, fmt.Sprintf("Subscribe(%v)", symbols), func() { err = h.Subscribe(c, symbols) })
	return err
}
