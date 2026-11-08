//go:build all || m5 || m5_1

// Checkpoint 5.1: exponential backoff with jitter

package upstream_test

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/upstream"
)

func TestCheckpoint5_1_Backoff(t *testing.T) {
	const s = time.Second
	tests := []struct {
		name    string
		attempt int
		base    time.Duration
		max     time.Duration
		jitter  float64
		want    time.Duration
	}{
		{"first attempt, no jitter", 0, s, 30 * s, 0, 500 * time.Millisecond},
		{"first attempt, full jitter", 0, s, 30 * s, 1, s},
		{"second attempt doubles", 1, s, 30 * s, 1, 2 * s},
		{"fourth attempt, no jitter", 3, s, 30 * s, 0, 4 * s},
		{"fourth attempt, half jitter", 3, s, 30 * s, 0.5, 6 * s},
		{"fourth attempt, full jitter", 3, s, 30 * s, 1, 8 * s},
		{"capped at max", 10, s, 30 * s, 1, 30 * s},
		{"capped, no jitter", 10, s, 30 * s, 0, 15 * s},
		{"huge attempt does not overflow", 200, s, 30 * s, 1, 30 * s},
		{"negative attempt treated as 0", -3, s, 30 * s, 1, s},
		{"small base", 2, 100 * time.Millisecond, 10 * s, 1, 400 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := upstream.Backoff(tc.attempt, tc.base, tc.max, tc.jitter)
			if got != tc.want {
				t.Fatalf("Backoff(attempt=%d, base=%v, max=%v, jitter=%v) = %v, want %v",
					tc.attempt, tc.base, tc.max, tc.jitter, got, tc.want)
			}
		})
	}
}

func TestCheckpoint5_1_BackoffNeverExceedsMax(t *testing.T) {
	for attempt := 0; attempt < 100; attempt++ {
		for _, j := range []float64{0, 0.3, 0.999, 1} {
			got := upstream.Backoff(attempt, 250*time.Millisecond, 20*time.Second, j)
			if got <= 0 || got > 20*time.Second {
				t.Fatalf("Backoff(%d, 250ms, 20s, %v) = %v; it must be > 0 and <= max", attempt, j, got)
			}
		}
	}
}
