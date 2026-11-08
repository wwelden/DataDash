//go:build all || m4 || m4_6

// Checkpoint 4.6: upstream status broadcast

package hub_test

import (
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_6_StatusGoesToEveryone(t *testing.T) {
	h := startHub(t, 0, 0)
	subscribed, idle := h.register(t), h.register(t)
	subscribe(t, h, "subscribed", subscribed, "AAPL")

	testkit.Within(t, 2*time.Second, "SetUpstreamStatus", func() { h.SetUpstreamStatus("reconnecting") })

	if m := next(t, "a subscribed client", subscribed); m["type"] != "status" || m["upstream"] != "reconnecting" {
		t.Fatalf("a subscribed client got %v, want {\"type\":\"status\",\"upstream\":\"reconnecting\"} (use protocol.EncodeStatus)", m)
	}
	if m := next(t, "a client with no subscriptions", idle); m["type"] != "status" || m["upstream"] != "reconnecting" {
		t.Fatalf("a client with no subscriptions got %v, want the status message too: status goes to every client", m)
	}
}

func TestCheckpoint4_6_StatusAfterStopIsSafe(t *testing.T) {
	h := startHub(t, 0, 0)
	h.register(t)
	h.stop(t)
	testkit.Within(t, time.Second, "SetUpstreamStatus after the hub stopped", func() { h.SetUpstreamStatus("connected") })
	testkit.Within(t, time.Second, "PublishQuote after the hub stopped", func() { h.PublishQuote(quote("AAPL", 1)) })
}
