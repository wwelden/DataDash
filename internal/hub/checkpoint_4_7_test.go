//go:build all || m4 || m4_7

// Checkpoint 4.7: 100 clients under load

package hub_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
)

func TestCheckpoint4_7_HundredClients(t *testing.T) {
	h := startHub(t, 0, 0)
	symbols := make([]string, 10)
	for i := range symbols {
		symbols[i] = fmt.Sprintf("SYM%c", 'A'+i)
	}
	const perSymbol = 100

	type result struct {
		client int
		err    string
	}
	results := make(chan result, 100)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		c := h.register(t)
		want := []string{symbols[i%10]}
		if i%4 == 0 {
			want = append(want, symbols[(i+5)%10])
		}
		subscribe(t, h, fmt.Sprintf("client %d", i), c, want...)

		wg.Add(1)
		go func(i int, c *hub.Client, want []string) {
			defer wg.Done()
			wanted := map[string]bool{}
			for _, s := range want {
				wanted[s] = true
			}
			last := map[string]float64{}
			got := 0
			timeout := time.After(10 * time.Second)
			for got < perSymbol*len(want) {
				select {
				case b, ok := <-c.Messages():
					if !ok {
						results <- result{i, "its Messages channel was closed (was it evicted?)"}
						return
					}
					var m struct {
						Symbol string  `json:"symbol"`
						Bid    float64 `json:"bid"`
					}
					json.Unmarshal(b, &m)
					if !wanted[m.Symbol] {
						results <- result{i, fmt.Sprintf("received %s, which it never subscribed to (it wanted %v)", m.Symbol, want)}
						return
					}
					if m.Bid <= last[m.Symbol] {
						results <- result{i, fmt.Sprintf("received %s quotes out of order (%v after %v)", m.Symbol, m.Bid, last[m.Symbol])}
						return
					}
					last[m.Symbol] = m.Bid
					got++
				case <-timeout:
					results <- result{i, fmt.Sprintf("only received %d of %d messages in 10s", got, perSymbol*len(want))}
					return
				}
			}
		}(i, c, want)
	}

	start := time.Now()
	for n := 1; n <= perSymbol; n++ {
		for _, s := range symbols {
			h.PublishQuote(market.Quote{Symbol: s, BidPrice: float64(n), AskPrice: float64(n) + 0.01, BidSize: 1, AskSize: 1, Timestamp: ts0})
		}
	}
	wg.Wait()
	elapsed := time.Since(start)
	close(results)
	for r := range results {
		t.Errorf("client %d %s", r.client, r.err)
	}
	if !t.Failed() {
		delivered := 0
		for i := 0; i < 100; i++ {
			delivered += perSymbol
			if i%4 == 0 {
				delivered += perSymbol
			}
		}
		t.Logf("published %d quotes, delivered %d messages to 100 clients in %v (%.0f deliveries/sec)",
			perSymbol*len(symbols), delivered, elapsed.Round(time.Millisecond), float64(delivered)/elapsed.Seconds())
	}
}
