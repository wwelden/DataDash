//go:build all || m0 || m0_1

// Checkpoint 0.1: market data types

package market_test

import (
	"math"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/market"
)

func TestCheckpoint0_1_QuoteFields(t *testing.T) {
	ts := time.Date(2026, 11, 6, 18, 30, 0, 123_000_000, time.UTC)
	q := market.Quote{
		Symbol:    "AAPL",
		BidPrice:  187.10,
		BidSize:   2,
		AskPrice:  187.12,
		AskSize:   3,
		Timestamp: ts,
	}
	if q.Symbol != "AAPL" || q.BidSize != 2 || q.AskSize != 3 || !q.Timestamp.Equal(ts) {
		t.Fatalf("Quote fields didn't hold their values: %+v", q)
	}
}

func TestCheckpoint0_1_TradeFields(t *testing.T) {
	ts := time.Date(2026, 11, 6, 18, 30, 0, 125_000_000, time.UTC)
	tr := market.Trade{Symbol: "AAPL", Price: 187.11, Size: 100, Timestamp: ts}
	if tr.Symbol != "AAPL" || tr.Price != 187.11 || tr.Size != 100 || !tr.Timestamp.Equal(ts) {
		t.Fatalf("Trade fields didn't hold their values: %+v", tr)
	}
}

func TestCheckpoint0_1_SpreadAndMid(t *testing.T) {
	tests := []struct {
		name     string
		bid, ask float64
		spread   float64
		mid      float64
	}{
		{"normal market", 187.10, 187.12, 0.02, 187.11},
		{"locked market", 50, 50, 0, 50},
		{"wide spread", 10, 12, 2, 11},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := market.Quote{Symbol: "X", BidPrice: tc.bid, AskPrice: tc.ask}
			if got := q.Spread(); math.Abs(got-tc.spread) > 1e-9 {
				t.Errorf("Spread() with bid %v ask %v = %v, want %v (ask minus bid)", tc.bid, tc.ask, got, tc.spread)
			}
			if got := q.Mid(); math.Abs(got-tc.mid) > 1e-9 {
				t.Errorf("Mid() with bid %v ask %v = %v, want %v (halfway between bid and ask)", tc.bid, tc.ask, got, tc.mid)
			}
		})
	}
}
