//go:build all || m7 || m7_1

// Checkpoint 7.1: latency percentiles for the load test

package stats_test

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/stats"
)

func ms(ns ...int) []time.Duration {
	out := make([]time.Duration, len(ns))
	for i, n := range ns {
		out[i] = time.Duration(n) * time.Millisecond
	}
	return out
}

func TestCheckpoint7_1_Percentile(t *testing.T) {
	ten := ms(10, 1, 9, 2, 8, 3, 7, 4, 6, 5) // 1..10 shuffled
	tests := []struct {
		name    string
		samples []time.Duration
		p       float64
		want    time.Duration
	}{
		{"empty", nil, 50, 0},
		{"single sample", ms(42), 99, 42 * time.Millisecond},
		{"p50 of 1..10", ten, 50, 5 * time.Millisecond},
		{"p90 of 1..10", ten, 90, 9 * time.Millisecond},
		{"p99 of 1..10", ten, 99, 10 * time.Millisecond},
		{"p100 is the max", ten, 100, 10 * time.Millisecond},
		{"p0 is the min", ten, 0, 1 * time.Millisecond},
		{"p25 of 1..4", ms(4, 3, 2, 1), 25, 1 * time.Millisecond},
		{"p51 of 1..4", ms(4, 3, 2, 1), 51, 3 * time.Millisecond},
		{"above 100 clamps", ten, 150, 10 * time.Millisecond},
		{"below 0 clamps", ten, -5, 1 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stats.Percentile(tc.samples, tc.p); got != tc.want {
				t.Fatalf("Percentile(%v, %v) = %v, want %v (nearest-rank method)", tc.samples, tc.p, got, tc.want)
			}
		})
	}
}

func TestCheckpoint7_1_DoesNotModifyInput(t *testing.T) {
	in := ms(3, 1, 2)
	stats.Percentile(in, 50)
	if in[0] != 3*time.Millisecond || in[1] != time.Millisecond || in[2] != 2*time.Millisecond {
		t.Fatalf("Percentile reordered the caller's slice (now %v). Sort a copy instead.", in)
	}
}

func TestCheckpoint7_1_Summary(t *testing.T) {
	s := stats.Summarize(ms(10, 1, 9, 2, 8, 3, 7, 4, 6, 5))
	if s.Count != 10 || s.Min != time.Millisecond || s.Max != 10*time.Millisecond ||
		s.P50 != 5*time.Millisecond || s.P99 != 10*time.Millisecond || s.Mean != 5500*time.Microsecond {
		t.Fatalf("Summarize(1..10 ms) = %+v, want Count 10, Min 1ms, Max 10ms, Mean 5.5ms, P50 5ms, P99 10ms", s)
	}
	if z := stats.Summarize(nil); z != (stats.Summary{}) {
		t.Fatalf("Summarize(nil) = %+v, want the zero Summary", z)
	}
}
