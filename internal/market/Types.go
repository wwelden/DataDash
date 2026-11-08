package market

import (
	"math"
	"time"
)

type Quote struct {
	Symbol    string
	BidPrice  float64
	BidSize   int
	AskPrice  float64
	AskSize   int
	Timestamp time.Time
}

type Trade struct {
	Symbol    string
	Price     float64
	Size      int
	Timestamp time.Time
}

func (q Quote) Spread() float64 {
	return math.Abs(q.BidPrice - q.AskPrice)
}
func (q Quote) Mid() float64 {
	return (q.AskPrice + q.BidPrice) / 2
}
