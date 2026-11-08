//go:build all || m4 || m4_2

// Checkpoint 4.2: subscriptions and fan-out

package hub_test

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_2_OnlySubscribedSymbols(t *testing.T) {
	h := startHub(t, 0, 0)
	a, b, c := h.register(t), h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	subscribe(t, h, "B", b, "MSFT")
	subscribe(t, h, "C", c, "AAPL", "MSFT")

	publishQuote(t, h, quote("AAPL", 1))
	publishQuote(t, h, quote("MSFT", 2))
	publishQuote(t, h, quote("AAPL", 3))
	publishQuote(t, h, quote("TSLA", 4)) // nobody subscribed

	expectQuote(t, "A", a, "AAPL", 1)
	expectQuote(t, "A", a, "AAPL", 3)
	expectQuote(t, "B", b, "MSFT", 2)
	expectQuote(t, "C", c, "AAPL", 1)
	expectQuote(t, "C", c, "MSFT", 2)
	expectQuote(t, "C", c, "AAPL", 3)

	// Sentinels: if anything extra was queued, it shows up before these.
	publishQuote(t, h, quote("AAPL", 99))
	publishQuote(t, h, quote("MSFT", 98))
	expectQuote(t, "A (after everything else)", a, "AAPL", 99)
	expectQuote(t, "B (after everything else)", b, "MSFT", 98)
}

func TestCheckpoint4_2_MessageFormat(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	publishQuote(t, h, market.Quote{Symbol: "AAPL", BidPrice: 187.10, BidSize: 2, AskPrice: 187.12, AskSize: 3, Timestamp: ts0})
	q := next(t, "A", a)
	want := map[string]any{"type": "quote", "symbol": "AAPL", "bid": 187.10, "bidSize": 2.0, "ask": 187.12, "askSize": 3.0}
	for k, v := range want {
		if q[k] != v {
			t.Errorf("quote message %v: %q = %#v, want %#v (use protocol.EncodeQuote)", q, k, q[k], v)
		}
	}
	testkit.Within(t, 2*time.Second, "PublishTrade", func() {
		h.PublishTrade(market.Trade{Symbol: "AAPL", Price: 187.11, Size: 100, Timestamp: ts0})
	})
	tr := next(t, "A", a)
	wantTrade := map[string]any{"type": "trade", "symbol": "AAPL", "price": 187.11, "size": 100.0}
	for k, v := range wantTrade {
		if tr[k] != v {
			t.Errorf("trade message %v: %q = %#v, want %#v (use protocol.EncodeTrade)", tr, k, tr[k], v)
		}
	}
}

func TestCheckpoint4_2_SubscribeTwiceIsIdempotent(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	subscribe(t, h, "A", a, "AAPL")
	if got := h.stats(t).Subscriptions["AAPL"]; got != 1 {
		t.Fatalf("one client subscribing to AAPL twice: Stats().Subscriptions[\"AAPL\"] = %d, want 1", got)
	}
	publishQuote(t, h, quote("AAPL", 1))
	publishQuote(t, h, quote("AAPL", 2))
	expectQuote(t, "A", a, "AAPL", 1)
	expectQuote(t, "A (should not get duplicates)", a, "AAPL", 2)
}

func TestCheckpoint4_2_Unsubscribe(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")
	unsubscribe(t, h, "A", a, "AAPL")
	publishQuote(t, h, quote("AAPL", 1))
	publishQuote(t, h, quote("MSFT", 2))
	expectQuote(t, "A (unsubscribed from AAPL)", a, "MSFT", 2)

	s := h.stats(t)
	if _, ok := s.Subscriptions["AAPL"]; ok {
		t.Fatalf("after the only subscriber left, Stats().Subscriptions still has AAPL: %v. Delete symbols that reach 0.", s.Subscriptions)
	}
	if s.Subscriptions["MSFT"] != 1 {
		t.Fatalf("Stats().Subscriptions = %v, want map[MSFT:1]", s.Subscriptions)
	}
	// Unsubscribing from something you never subscribed to is not an error.
	unsubscribe(t, h, "A", a, "NFLX")
}

func TestCheckpoint4_2_RefCountsInStats(t *testing.T) {
	h := startHub(t, 0, 0)
	a, b, c := h.register(t), h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	subscribe(t, h, "B", b, "AAPL", "MSFT")
	subscribe(t, h, "C", c, "AAPL")
	s := h.stats(t)
	if s.Subscriptions["AAPL"] != 3 || s.Subscriptions["MSFT"] != 1 || len(s.Subscriptions) != 2 {
		t.Fatalf("Stats().Subscriptions = %v, want map[AAPL:3 MSFT:1]", s.Subscriptions)
	}
	testkit.Within(t, 2*time.Second, "Unregister", func() { h.Unregister(b) })
	s = h.stats(t)
	if s.Subscriptions["AAPL"] != 2 || len(s.Subscriptions) != 1 {
		t.Fatalf("after B unregistered, Stats().Subscriptions = %v, want map[AAPL:2]. "+
			"Unregistering must also drop the client's subscriptions.", s.Subscriptions)
	}
}

func TestCheckpoint4_2_StatsIsACopy(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	s := h.stats(t)
	if s.Subscriptions == nil {
		t.Fatal("Stats().Subscriptions is nil")
	}
	s.Subscriptions["AAPL"] = 1000
	s.Subscriptions["HACK"] = 1
	if got := h.stats(t).Subscriptions; got["AAPL"] != 1 || len(got) != 1 {
		t.Fatalf("changing the map returned by Stats() changed the hub's state (%v).\n"+
			"Return a copy: handing the hub's own map to another goroutine is a data race.", got)
	}
}

func TestCheckpoint4_2_KeepsOrder(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	go func() {
		for i := 0; i < 100; i++ {
			h.PublishQuote(quote("AAPL", float64(i)))
		}
	}()
	for i := 0; i < 100; i++ {
		expectQuote(t, "A", a, "AAPL", float64(i))
	}
}

func TestCheckpoint4_2_SubscribeUnknownClient(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	testkit.Within(t, 2*time.Second, "Unregister", func() { h.Unregister(a) })
	var err error
	testkit.Within(t, 2*time.Second, "Subscribe with an unregistered client", func() {
		err = h.Subscribe(a, []string{"AAPL"})
	})
	if err == nil {
		t.Fatal("Subscribe for a client that was unregistered should return an error")
	}
	if got := h.stats(t).Subscriptions; len(got) != 0 {
		t.Fatalf("an unregistered client's Subscribe changed the hub's state: %v", got)
	}
}
