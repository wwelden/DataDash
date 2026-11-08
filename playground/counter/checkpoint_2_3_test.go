//go:build all || m2 || m2_3

// Checkpoint 2.3: fix the race with sync/atomic

package main

import "testing"

func TestCheckpoint2_3_AtomicCounter(t *testing.T) {
	var c Counter = NewAtomicCounter()
	hammer(t, "NewAtomicCounter()", c)
}
