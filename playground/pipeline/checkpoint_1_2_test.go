//go:build all || m1 || m1_2

// Checkpoint 1.2: pipeline and fan-in

package main

import (
	"sort"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

// drain reads ch until it's closed, failing if that takes longer than d.
func drain(t *testing.T, name string, ch <-chan int, d time.Duration) []int {
	t.Helper()
	var out []int
	timeout := time.After(d)
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, v)
		case <-timeout:
			t.Fatalf("%s: got %v so far, but the channel was never closed.\n"+
				"The goroutine that sends on a channel should close it when it's done (defer close(out)).", name, out)
		}
	}
}

func TestCheckpoint1_2_Generate(t *testing.T) {
	got := drain(t, "Generate(1, 2, 3)", Generate(1, 2, 3), 2*time.Second)
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("Generate(1, 2, 3) produced %v, want [1 2 3] in order", got)
	}
	if got := drain(t, "Generate()", Generate(), 2*time.Second); len(got) != 0 {
		t.Fatalf("Generate() with no numbers produced %v, want nothing", got)
	}
}

func TestCheckpoint1_2_GenerateReturnsImmediately(t *testing.T) {
	var ch <-chan int
	testkit.Within(t, time.Second, "Generate(1..1000) returning its channel", func() {
		nums := make([]int, 1000)
		ch = Generate(nums...)
	})
	drain(t, "Generate(1..1000)", ch, 2*time.Second)
}

func TestCheckpoint1_2_SquareKeepsOrder(t *testing.T) {
	got := drain(t, "Square(Generate(1..5))", Square(Generate(1, 2, 3, 4, 5)), 2*time.Second)
	want := []int{1, 4, 9, 16, 25}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (in order)", got, want)
		}
	}
}

func TestCheckpoint1_2_ChainedSquares(t *testing.T) {
	got := drain(t, "Square(Square(Generate(2, 3)))", Square(Square(Generate(2, 3))), 2*time.Second)
	if len(got) != 2 || got[0] != 16 || got[1] != 81 {
		t.Fatalf("got %v, want [16 81]", got)
	}
}

func TestCheckpoint1_2_MergeFanIn(t *testing.T) {
	in := Generate(1, 2, 3, 4, 5, 6, 7, 8, 9)
	// three workers all reading from the same input: fan-out
	a, b, c := Square(in), Square(in), Square(in)
	got := drain(t, "Merge(a, b, c)", Merge(a, b, c), 3*time.Second)
	sort.Ints(got)
	want := []int{1, 4, 9, 16, 25, 36, 49, 64, 81}
	if len(got) != len(want) {
		t.Fatalf("Merge produced %d values %v, want %d values %v (any order)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Merge produced %v, want %v (any order)", got, want)
		}
	}
}

func TestCheckpoint1_2_MergeOfNothing(t *testing.T) {
	drain(t, "Merge() with no inputs", Merge(), time.Second)
}

func TestCheckpoint1_2_NoGoroutinesLeftBehind(t *testing.T) {
	for i := 0; i < 3; i++ {
		drain(t, "pipeline", Merge(Square(Generate(1, 2, 3)), Square(Generate(4, 5))), 2*time.Second)
	}
	testkit.NoLeaks(t, 2*time.Second, "main.")
}
