//go:build all || m2 || m2_2 || m2_3 || m2_4

// Shared helpers for checkpoints 2.2 to 2.4.

package main

import (
	"sync"
	"testing"
)

func hammer(t *testing.T, name string, c Counter) {
	t.Helper()
	if c == nil {
		t.Fatalf("%s returned nil", name)
	}
	if v := c.Value(); v != 0 {
		t.Fatalf("a new counter from %s should start at 0, got %d", name, v)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Inc()
				if j%100 == 0 {
					c.Value() // reads race with writes too
				}
			}
		}()
	}
	wg.Wait()
	if v := c.Value(); v != 100_000 {
		t.Fatalf("%s: 100 goroutines x 1000 increments gave %d, want 100000", name, v)
	}
}
