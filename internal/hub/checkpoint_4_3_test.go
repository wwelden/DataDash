//go:build all || m4 || m4_3

// Checkpoint 4.3: reference-counted upstream subscriptions

package hub_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_3_FirstSubscriberOpensUpstream(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")
	h.up.waitActive(t, "AAPL", "MSFT")
}

func TestCheckpoint4_3_SharedSymbolOpensOnce(t *testing.T) {
	h := startHub(t, 0, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	subscribe(t, h, "B", b, "AAPL", "MSFT")
	h.up.waitActive(t, "AAPL", "MSFT")
	if n := h.up.Subs("AAPL"); n != 1 {
		t.Fatalf("AAPL was subscribed upstream %d times; the second client should reuse the existing subscription", n)
	}
}

func TestCheckpoint4_3_LastSubscriberClosesUpstream(t *testing.T) {
	h := startHub(t, 0, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL")
	subscribe(t, h, "B", b, "AAPL")
	h.up.waitActive(t, "AAPL")

	unsubscribe(t, h, "A", a, "AAPL")
	time.Sleep(50 * time.Millisecond)
	if n := h.up.Unsubs("AAPL"); n != 0 {
		t.Fatal("AAPL was unsubscribed upstream while client B still wanted it")
	}
	unsubscribe(t, h, "B", b, "AAPL")
	h.up.waitActive(t)
}

func TestCheckpoint4_3_UnregisterReleasesSymbols(t *testing.T) {
	h := startHub(t, 0, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")
	subscribe(t, h, "B", b, "MSFT")
	h.up.waitActive(t, "AAPL", "MSFT")

	testkit.Within(t, 2*time.Second, "Unregister(A)", func() { h.Unregister(a) })
	h.up.waitActive(t, "MSFT")
	testkit.Within(t, 2*time.Second, "Unregister(B)", func() { h.Unregister(b) })
	h.up.waitActive(t)
}

func TestCheckpoint4_3_ResubscribeAfterRelease(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	for i := 0; i < 5; i++ {
		subscribe(t, h, "A", a, "AAPL")
		unsubscribe(t, h, "A", a, "AAPL")
	}
	subscribe(t, h, "A", a, "AAPL")
	h.up.waitActive(t, "AAPL")
}

// Many clients subscribing and unsubscribing at once. The fake upstream
// complains if calls overlap, arrive out of order (double subscribe or
// unsubscribing something that isn't subscribed), or exceed the cap.
func TestCheckpoint4_3_Churn(t *testing.T) {
	h := startHub(t, 0, 0)
	symbols := []string{"AAPL", "MSFT", "GOOG", "TSLA", "NVDA"}
	var wg sync.WaitGroup
	clients := make([]*hub.Client, 10)
	for i := range clients {
		clients[i] = h.register(t)
	}
	testkit.Within(t, 20*time.Second, "10 clients each doing 40 random subscribes and unsubscribes", func() {
		for i, c := range clients {
			wg.Add(1)
			go func(i int, c *hub.Client) {
				defer wg.Done()
				r := rand.New(rand.NewSource(int64(i)))
				for j := 0; j < 40; j++ {
					s := []string{symbols[r.Intn(len(symbols))]}
					var err error
					if r.Intn(2) == 0 {
						err = h.Subscribe(c, s)
					} else {
						err = h.Unsubscribe(c, s)
					}
					if err != nil {
						t.Errorf("client %d: %v", i, err)
						return
					}
				}
			}(i, c)
		}
		wg.Wait()
	})

	// Whatever the hub thinks is subscribed, upstream must match.
	want := []string{}
	for sym := range h.stats(t).Subscriptions {
		want = append(want, sym)
	}
	h.up.waitActive(t, want...)

	for i, c := range clients {
		testkit.Within(t, 2*time.Second, fmt.Sprintf("Unregister(client %d)", i), func() { h.Unregister(c) })
	}
	h.up.waitActive(t)
}
