//go:build m2_1

// Checkpoint 2.1: a counter with a data race (this one is SUPPOSED to fail)

package main

import (
	"sync"
	"testing"
)

// This test exists to show you what a data race looks like. With -race
// (which ./check.sh always uses) it should FAIL with "WARNING: DATA RACE".
// Read the report: it shows the two goroutines touching the same memory.
// Without -race, look at how many increments were lost.
func TestCheckpoint2_1_UnsafeCounterRaces(t *testing.T) {
	var c Counter = NewUnsafeCounter()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Inc()
			}
		}()
	}
	wg.Wait()
	t.Logf("UnsafeCounter counted %d of 50000 increments (%d lost)", c.Value(), 50000-c.Value())
	if c.Value() != 50000 {
		t.Errorf("lost %d increments to the race", 50000-c.Value())
	}
}
