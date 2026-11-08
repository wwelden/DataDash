//go:build all || m1 || m1_3

// Checkpoint 1.3: select

package main

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint1_3_TrySendWithRoom(t *testing.T) {
	ch := make(chan int, 1)
	var ok bool
	testkit.Within(t, time.Second, "TrySend into a channel with room", func() { ok = TrySend(ch, 7) })
	if !ok {
		t.Fatal("TrySend returned false, but the channel had room")
	}
	if v := <-ch; v != 7 {
		t.Fatalf("channel received %d, want 7", v)
	}
}

func TestCheckpoint1_3_TrySendWhenFull(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 1
	var ok bool
	testkit.Within(t, time.Second, "TrySend into a full channel", func() { ok = TrySend(ch, 2) })
	if ok {
		t.Fatal("TrySend returned true, but the channel was full")
	}
	if v := <-ch; v != 1 {
		t.Fatalf("the channel should still hold the original value 1, got %d", v)
	}
}

func TestCheckpoint1_3_TrySendNoReceiver(t *testing.T) {
	ch := make(chan int) // unbuffered, nobody receiving
	var ok bool
	testkit.Within(t, time.Second, "TrySend into an unbuffered channel with no receiver", func() { ok = TrySend(ch, 1) })
	if ok {
		t.Fatal("TrySend returned true, but nobody could have received the value")
	}
}

func TestCheckpoint1_3_RecvTimeoutGetsValue(t *testing.T) {
	ch := make(chan int, 1)
	ch <- 42
	var v int
	var ok bool
	testkit.Within(t, time.Second, "RecvTimeout on a channel holding a value", func() {
		v, ok = RecvTimeout(ch, 500*time.Millisecond)
	})
	if !ok || v != 42 {
		t.Fatalf("RecvTimeout = (%d, %v), want (42, true)", v, ok)
	}
}

func TestCheckpoint1_3_RecvTimeoutWaitsForLateValue(t *testing.T) {
	ch := make(chan int)
	go func() {
		time.Sleep(50 * time.Millisecond)
		ch <- 9
	}()
	var v int
	var ok bool
	testkit.Within(t, 2*time.Second, "RecvTimeout", func() { v, ok = RecvTimeout(ch, time.Second) })
	if !ok || v != 9 {
		t.Fatalf("RecvTimeout = (%d, %v), want (9, true): the value arrived before the timeout", v, ok)
	}
}

func TestCheckpoint1_3_RecvTimeoutTimesOut(t *testing.T) {
	ch := make(chan int)
	start := time.Now()
	var v int
	var ok bool
	testkit.Within(t, 2*time.Second, "RecvTimeout(100ms) on a silent channel", func() {
		v, ok = RecvTimeout(ch, 100*time.Millisecond)
	})
	elapsed := time.Since(start)
	if ok || v != 0 {
		t.Fatalf("RecvTimeout = (%d, %v), want (0, false) after timing out", v, ok)
	}
	if elapsed < 90*time.Millisecond {
		t.Fatalf("RecvTimeout returned after %v, but should wait the full 100ms", elapsed)
	}
}

func TestCheckpoint1_3_RecvTimeoutClosedChannel(t *testing.T) {
	ch := make(chan int)
	close(ch)
	start := time.Now()
	var v int
	var ok bool
	testkit.Within(t, 2*time.Second, "RecvTimeout on a closed channel", func() {
		v, ok = RecvTimeout(ch, time.Second)
	})
	if ok || v != 0 {
		t.Fatalf("RecvTimeout on a closed channel = (%d, %v), want (0, false).\n"+
			"Hint: v, ok := <-ch tells you whether the channel was closed.", v, ok)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("RecvTimeout on a closed channel should return right away, not wait for the timeout")
	}
}
