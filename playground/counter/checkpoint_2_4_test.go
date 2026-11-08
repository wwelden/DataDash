//go:build all || m2 || m2_4

// Checkpoint 2.4: stopping goroutines with context

package main

import (
	"context"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint2_4_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := NewAtomicCounter()

	var wait func()
	testkit.Within(t, time.Second, "StartIncrementers", func() {
		wait = StartIncrementers(ctx, 8, c)
	})
	if wait == nil {
		t.Fatal("StartIncrementers returned a nil wait function")
	}

	testkit.Eventually(t, 2*time.Second, func() bool { return c.Value() > 0 },
		"after starting 8 incrementers, the counter never went above 0. Are the goroutines running?")

	cancel()
	testkit.Within(t, 2*time.Second, "wait() after cancelling the context", wait)

	stopped := c.Value()
	time.Sleep(50 * time.Millisecond)
	if after := c.Value(); after != stopped {
		t.Fatalf("the counter kept going after wait() returned (%d -> %d). "+
			"wait() should only return once every goroutine has exited.", stopped, after)
	}
	testkit.NoLeaks(t, time.Second, "main.")
}

func TestCheckpoint2_4_AlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wait func()
	testkit.Within(t, time.Second, "StartIncrementers with an already-cancelled context", func() {
		wait = StartIncrementers(ctx, 4, NewMutexCounter())
	})
	testkit.Within(t, time.Second, "wait() with an already-cancelled context", wait)
	testkit.NoLeaks(t, time.Second, "main.")
}

func TestCheckpoint2_4_DeadlineAlsoStops(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var wait func()
	testkit.Within(t, time.Second, "StartIncrementers", func() {
		wait = StartIncrementers(ctx, 4, NewAtomicCounter())
	})
	testkit.Within(t, 2*time.Second, "wait() with a context that times out after 50ms", wait)
}
