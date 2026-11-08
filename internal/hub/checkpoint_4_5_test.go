//go:build all || m4 || m4_5

// Checkpoint 4.5: slow clients get evicted

package hub_test

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_5_SlowClientIsEvicted(t *testing.T) {
	const queue = 8
	h := startHub(t, 0, queue)
	fast, slow := h.register(t), h.register(t)
	subscribe(t, h, "fast", fast, "AAPL")
	subscribe(t, h, "slow", slow, "AAPL", "TSLA")
	h.up.waitActive(t, "AAPL", "TSLA")

	// The fast client keeps up; the slow one never reads.
	price := 0.0
	for round := 0; round < 10; round++ {
		for i := 0; i < queue/2; i++ {
			price++
			publishQuote(t, h, quote("AAPL", price))
		}
		for i := 0; i < queue/2; i++ {
			want := float64(round*queue/2 + i + 1)
			expectQuote(t, "the fast client", fast, "AAPL", want)
		}
	}

	got := expectClosed(t, "the slow client", slow, 2*time.Second)
	if got > queue {
		t.Fatalf("the slow client had %d messages queued, more than QueueSize %d. The queue should be a channel with that buffer size.", got, queue)
	}
	testkit.Eventually(t, 2*time.Second, func() bool { return h.Stats().Clients == 1 },
		"after evicting the slow client, Stats().Clients should be 1")
	if n := h.stats(t).Subscriptions["AAPL"]; n != 1 {
		t.Fatalf("after evicting the slow client, Stats().Subscriptions[\"AAPL\"] = %d, want 1", n)
	}
	// TSLA was only wanted by the evicted client, so it should be released upstream.
	h.up.waitActive(t, "AAPL")

	// The slow client's code will eventually notice and clean up. That must be safe.
	testkit.Within(t, 2*time.Second, "Unregister of an already-evicted client", func() { h.Unregister(slow) })
	if err := trySubscribe(t, h, slow, "AAPL"); err == nil {
		t.Fatal("Subscribe for an evicted client should return an error")
	}
}

func TestCheckpoint4_5_PublishNeverBlocksOnASlowClient(t *testing.T) {
	h := startHub(t, 0, 4)
	stalled := h.register(t)
	subscribe(t, h, "stalled", stalled, "AAPL")
	testkit.Within(t, 3*time.Second, "publishing 1000 quotes while one subscriber never reads", func() {
		for i := 0; i < 1000; i++ {
			h.PublishQuote(quote("AAPL", float64(i)))
		}
	})
}

func TestCheckpoint4_5_DefaultQueueSize(t *testing.T) {
	h := startHub(t, 0, 0) // 0 means the default of 256
	c := h.register(t)
	subscribe(t, h, "c", c, "AAPL")
	for i := 0; i < 200; i++ {
		publishQuote(t, h, quote("AAPL", float64(i)))
	}
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 200; i++ {
		expectQuote(t, "a client that read after 200 quotes (default QueueSize should be 256)", c, "AAPL", float64(i))
	}
}
