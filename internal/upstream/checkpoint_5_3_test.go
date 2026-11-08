//go:build all || m5 || m5_3

// Checkpoint 5.3: live data from Alpaca's test stream
//
// These tests talk to Alpaca for real. They are skipped unless
// APCA_API_KEY_ID and APCA_API_SECRET_KEY are set (check.sh loads .env).

package upstream_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/upstream"
)

// Your client must be usable as the hub's upstream.
var _ hub.Upstream = (*upstream.Client)(nil)

// The hub must be usable as the client's publisher.
var _ upstream.Publisher = (*hub.Hub)(nil)

type pub struct {
	mu       sync.Mutex
	quotes   []market.Quote
	trades   []market.Trade
	statuses []string
}

func (p *pub) PublishQuote(q market.Quote) {
	p.mu.Lock()
	p.quotes = append(p.quotes, q)
	p.mu.Unlock()
}
func (p *pub) PublishTrade(t market.Trade) {
	p.mu.Lock()
	p.trades = append(p.trades, t)
	p.mu.Unlock()
}
func (p *pub) SetUpstreamStatus(s string) {
	p.mu.Lock()
	p.statuses = append(p.statuses, s)
	p.mu.Unlock()
}

func keys(t *testing.T) (string, string) {
	t.Helper()
	id, secret := os.Getenv("APCA_API_KEY_ID"), os.Getenv("APCA_API_SECRET_KEY")
	if id == "" || secret == "" {
		t.Skip("APCA_API_KEY_ID / APCA_API_SECRET_KEY not set; put them in .env to run the live Alpaca checks")
	}
	return id, secret
}

func TestCheckpoint5_3_SubscribeBeforeConnectFails(t *testing.T) {
	c := upstream.New(upstream.Config{KeyID: "x", SecretKey: "y", Feed: "test"})
	if err := c.Subscribe([]string{"FAKEPACA"}); err == nil {
		t.Fatal("Subscribe before Connect should return an error, not panic or succeed")
	}
}

func TestCheckpoint5_3_LiveTestStream(t *testing.T) {
	id, secret := keys(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	p := &pub{}
	c := upstream.New(upstream.Config{KeyID: id, SecretKey: secret, Feed: "test"})
	if err := c.Connect(ctx, p); err != nil {
		t.Fatalf("Connect to the test stream failed: %v", err)
	}
	if err := c.Subscribe([]string{"FAKEPACA"}); err != nil {
		t.Fatalf("Subscribe(FAKEPACA) failed: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		n := len(p.quotes)
		var first market.Quote
		if n > 0 {
			first = p.quotes[0]
		}
		p.mu.Unlock()
		if n > 0 {
			if first.Symbol != "FAKEPACA" || first.BidPrice <= 0 {
				t.Fatalf("got quote %+v; want a FAKEPACA quote with a positive bid", first)
			}
			t.Logf("received %d FAKEPACA quotes; first: %+v", n, first)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.quotes) == 0 {
		t.Fatal("no FAKEPACA quotes arrived within 30s of subscribing to the test stream")
	}
	gotConnected := false
	for _, s := range p.statuses {
		if s == "connected" {
			gotConnected = true
		}
	}
	if !gotConnected {
		t.Errorf("the client should call SetUpstreamStatus(\"connected\") once connected; statuses seen: %v", p.statuses)
	}
	if err := c.Unsubscribe([]string{"FAKEPACA"}); err != nil {
		t.Errorf("Unsubscribe(FAKEPACA) failed: %v", err)
	}
}

func TestCheckpoint5_3_BadKeysFailFast(t *testing.T) {
	// Needs the network, but not your keys.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := upstream.New(upstream.Config{KeyID: "PKBADBADBADBADBADBAD", SecretKey: "nope", Feed: "test"})
	start := time.Now()
	err := c.Connect(ctx, &pub{})
	if err == nil {
		t.Fatal("Connect with invalid keys should return an error")
	}
	if errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 20*time.Second {
		t.Fatalf("Connect with invalid keys took %v (%v). Auth failure is fatal: don't retry it forever.", time.Since(start), err)
	}
}
