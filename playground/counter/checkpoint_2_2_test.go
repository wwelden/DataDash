//go:build all || m2 || m2_2

// Checkpoint 2.2: fix the race with a mutex

package main

import "testing"

func TestCheckpoint2_2_MutexCounter(t *testing.T) {
	var c Counter = NewMutexCounter()
	hammer(t, "NewMutexCounter()", c)
}
