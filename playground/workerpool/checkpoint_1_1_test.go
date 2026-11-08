//go:build all || m1 || m1_1

// Checkpoint 1.1: worker pool

package main

import (
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func square(n int) int { return n * n }

func TestCheckpoint1_1_ThousandJobsFiveWorkers(t *testing.T) {
	jobs := make([]int, 1000)
	for i := range jobs {
		jobs[i] = i + 1
	}
	var results []int
	testkit.Within(t, 10*time.Second, "RunPool(1000 jobs, 5 workers)", func() {
		results = RunPool(jobs, 5, square)
	})
	if len(results) != 1000 {
		t.Fatalf("got %d results, want 1000. Results were lost or duplicated.", len(results))
	}
	sum := 0
	for _, r := range results {
		sum += r
	}
	if sum != 333_833_500 {
		t.Fatalf("sum of results = %d, want 333833500 (the sum of the squares of 1..1000)", sum)
	}
	sort.Ints(results)
	for i, r := range results {
		if r != (i+1)*(i+1) {
			t.Fatalf("result %d^2 is missing or wrong (sorted results[%d] = %d)", i+1, i, r)
		}
	}
}

func TestCheckpoint1_1_UsesTheWorkers(t *testing.T) {
	var running, maxRunning atomic.Int64
	work := func(n int) int {
		now := running.Add(1)
		for {
			m := maxRunning.Load()
			if now <= m || maxRunning.CompareAndSwap(m, now) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		running.Add(-1)
		return n
	}
	jobs := make([]int, 200)
	testkit.Within(t, 10*time.Second, "RunPool(200 jobs, 4 workers)", func() {
		RunPool(jobs, 4, work)
	})
	switch m := maxRunning.Load(); {
	case m > 4:
		t.Fatalf("up to %d jobs ran at the same time, but only 4 workers were asked for", m)
	case m < 2:
		t.Fatalf("jobs never ran at the same time; RunPool seems to work through them one by one.\n" +
			"Start each worker with the go keyword so they run concurrently.")
	}
}

func TestCheckpoint1_1_EdgeCases(t *testing.T) {
	t.Run("no jobs", func(t *testing.T) {
		var res []int
		testkit.Within(t, 2*time.Second, "RunPool with an empty job list", func() {
			res = RunPool(nil, 3, square)
		})
		if len(res) != 0 {
			t.Fatalf("got %d results for zero jobs", len(res))
		}
	})
	t.Run("more workers than jobs", func(t *testing.T) {
		var res []int
		testkit.Within(t, 2*time.Second, "RunPool(3 jobs, 10 workers)", func() {
			res = RunPool([]int{1, 2, 3}, 10, square)
		})
		sort.Ints(res)
		if len(res) != 3 || res[0] != 1 || res[1] != 4 || res[2] != 9 {
			t.Fatalf("got %v, want [1 4 9] in any order", res)
		}
	})
	t.Run("zero workers means one", func(t *testing.T) {
		var res []int
		testkit.Within(t, 2*time.Second, "RunPool(3 jobs, 0 workers)", func() {
			res = RunPool([]int{1, 2, 3}, 0, square)
		})
		if len(res) != 3 {
			t.Fatalf("with workers < 1, RunPool should use 1 worker; got %d results, want 3", len(res))
		}
	})
}

func TestCheckpoint1_1_NoGoroutinesLeftBehind(t *testing.T) {
	for i := 0; i < 5; i++ {
		testkit.Within(t, 5*time.Second, "RunPool", func() {
			RunPool([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 3, square)
		})
	}
	testkit.NoLeaks(t, 2*time.Second, "main.")
}
