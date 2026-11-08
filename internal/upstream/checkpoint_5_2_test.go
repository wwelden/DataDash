//go:build all || m5 || m5_2

// Checkpoint 5.2: mapping Alpaca's types to yours

package upstream_test

import (
	"testing"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"

	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/upstream"
)

func TestCheckpoint5_2_QuoteFromAlpaca(t *testing.T) {
	ny := time.FixedZone("EST", -5*60*60)
	ts := time.Date(2026, 11, 6, 13, 30, 0, 639713735, ny)
	in := stream.Quote{
		Symbol: "FAKEPACA", BidExchange: "O", BidPrice: 133.85, BidSize: 4,
		AskExchange: "R", AskPrice: 135.77, AskSize: 5, Timestamp: ts,
		Conditions: []string{"R"}, Tape: "A",
	}
	got := upstream.QuoteFromAlpaca(in)
	want := market.Quote{Symbol: "FAKEPACA", BidPrice: 133.85, BidSize: 4, AskPrice: 135.77, AskSize: 5, Timestamp: ts.UTC()}
	if got != want {
		t.Fatalf("QuoteFromAlpaca:\n got %+v\nwant %+v", got, want)
	}
	if got.Timestamp.Location() != time.UTC {
		t.Fatalf("Timestamp should be converted to UTC, got location %v", got.Timestamp.Location())
	}
}

func TestCheckpoint5_2_TradeFromAlpaca(t *testing.T) {
	ts := time.Date(2026, 11, 6, 18, 30, 0, 125_000_000, time.UTC)
	in := stream.Trade{ID: 52983525029461, Symbol: "AAPL", Exchange: "V", Price: 187.11, Size: 100, Timestamp: ts, Tape: "C"}
	got := upstream.TradeFromAlpaca(in)
	want := market.Trade{Symbol: "AAPL", Price: 187.11, Size: 100, Timestamp: ts}
	if got != want {
		t.Fatalf("TradeFromAlpaca:\n got %+v\nwant %+v", got, want)
	}
}
