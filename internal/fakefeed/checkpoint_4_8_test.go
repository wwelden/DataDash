//go:build all || m4 || m4_8

// Checkpoint 4.8: a fake upstream feed for offline development

package fakefeed_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/fakefeed"
	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/testkit"
)

// The feed must be usable as the hub's upstream.
var _ hub.Upstream = (*fakefeed.Feed)(nil)

type recorder struct {
	mu     sync.Mutex
	quotes []market.Quote
	trades []market.Trade
}

func (r *recorder) PublishQuote(q market.Quote) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.quotes = append(r.quotes, q)
}

func (r *recorder) PublishTrade(t market.Trade) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trades = append(r.trades, t)
}

func (r *recorder) counts(sym string) (quotes, trades int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, q := range r.quotes {
		if q.Symbol == sym {
			quotes++
		}
	}
	for _, t := range r.trades {
		if t.Symbol == sym {
			trades++
		}
	}
	return
}

func runFeed(t *testing.T, interval time.Duration) (*fakefeed.Feed, *recorder) {
	t.Helper()
	var f *fakefeed.Feed
	testkit.Within(t, time.Second, "fakefeed.New", func() { f = fakefeed.New(interval) })
	if f == nil {
		t.Fatal("fakefeed.New returned nil")
	}
	rec := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.Run(ctx, rec)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Feed.Run did not return within 2s after its context was cancelled")
		}
	})
	return f, rec
}

func TestCheckpoint4_8_PublishesSubscribedSymbols(t *testing.T) {
	f, rec := runFeed(t, 10*time.Millisecond)
	if err := f.Subscribe([]string{"AAPL", "MSFT"}); err != nil {
		t.Fatalf("Subscribe returned %v", err)
	}
	testkit.Eventually(t, 2*time.Second, func() bool {
		aq, at := rec.counts("AAPL")
		mq, mt := rec.counts("MSFT")
		return aq >= 3 && at >= 3 && mq >= 3 && mt >= 3
	}, "with a 10ms interval, expected at least 3 quotes and 3 trades each for AAPL and MSFT within 2s")

	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, q := range rec.quotes {
		if q.Symbol != "AAPL" && q.Symbol != "MSFT" {
			t.Fatalf("published a quote for %q, which nobody subscribed to", q.Symbol)
		}
		if q.BidPrice <= 0 || q.AskPrice < q.BidPrice || q.BidSize <= 0 || q.AskSize <= 0 {
			t.Fatalf("unrealistic quote %+v: want BidPrice > 0, AskPrice >= BidPrice, sizes > 0", q)
		}
		if time.Since(q.Timestamp) > 5*time.Second || q.Timestamp.After(time.Now().Add(time.Second)) {
			t.Fatalf("quote timestamp %v should be about now", q.Timestamp)
		}
	}
	for _, tr := range rec.trades {
		if tr.Price <= 0 || tr.Size <= 0 {
			t.Fatalf("unrealistic trade %+v: want Price > 0 and Size > 0", tr)
		}
	}
}

func TestCheckpoint4_8_PricesWalkGently(t *testing.T) {
	f, rec := runFeed(t, 5*time.Millisecond)
	f.Subscribe([]string{"AAPL"})
	testkit.Eventually(t, 2*time.Second, func() bool { q, _ := rec.counts("AAPL"); return q >= 20 },
		"expected 20 AAPL quotes")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	changed := false
	for i := 1; i < len(rec.quotes); i++ {
		prev, cur := rec.quotes[i-1].BidPrice, rec.quotes[i].BidPrice
		if cur != prev {
			changed = true
		}
		if cur > prev*1.05 || cur < prev*0.95 {
			t.Fatalf("price jumped from %v to %v in one step; make it a random walk with small steps", prev, cur)
		}
	}
	if !changed {
		t.Fatal("the price never moved across 20 quotes; make it a random walk")
	}
}

func TestCheckpoint4_8_UnsubscribeStops(t *testing.T) {
	f, rec := runFeed(t, 5*time.Millisecond)
	f.Subscribe([]string{"AAPL", "MSFT"})
	testkit.Eventually(t, 2*time.Second, func() bool { q, _ := rec.counts("MSFT"); return q >= 2 }, "expected MSFT quotes")
	if err := f.Unsubscribe([]string{"MSFT"}); err != nil {
		t.Fatalf("Unsubscribe returned %v", err)
	}
	time.Sleep(30 * time.Millisecond) // let an in-flight tick finish
	before, _ := rec.counts("MSFT")
	aaplBefore, _ := rec.counts("AAPL")
	time.Sleep(100 * time.Millisecond)
	after, _ := rec.counts("MSFT")
	aaplAfter, _ := rec.counts("AAPL")
	if after != before {
		t.Fatalf("after Unsubscribe(MSFT), %d more MSFT quotes were published", after-before)
	}
	if aaplAfter == aaplBefore {
		t.Fatal("after unsubscribing MSFT, AAPL quotes stopped too")
	}
}

func TestCheckpoint4_8_SubscribeFromOtherGoroutines(t *testing.T) {
	f, _ := runFeed(t, time.Millisecond)
	var wg sync.WaitGroup
	syms := []string{"AAPL", "MSFT", "GOOG", "TSLA"}
	testkit.Within(t, 5*time.Second, "concurrent Subscribe/Unsubscribe calls while Run is ticking", func() {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					s := []string{syms[(i+j)%len(syms)]}
					f.Subscribe(s)
					f.Unsubscribe(s)
				}
			}(i)
		}
		wg.Wait()
	})
}

func TestCheckpoint4_8_NoLeaks(t *testing.T) {
	f := fakefeed.New(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.Run(ctx, &recorder{})
	}()
	f.Subscribe([]string{"AAPL"})
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Feed.Run did not return after cancel")
	}
	testkit.NoLeaks(t, 2*time.Second, testkit.Module+"/internal/")
}
