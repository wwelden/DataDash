//go:build all || m4 || m4_9

// Checkpoint 4.9: browser sessions (WebSocket <-> hub)

package client_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/client"
	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_9_ConnectRegisters(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	testkit.DialWS(t, e.srv.URL, "/")
	testkit.DialWS(t, e.srv.URL, "/")
	e.waitClients(t, 2)
}

func TestCheckpoint4_9_SubscribeAndReceive(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["aapl"]}`)
	e.waitSubscribed(t, "AAPL", 1)

	e.hub.PublishQuote(q("MSFT", 1)) // not subscribed
	e.hub.PublishQuote(q("AAPL", 2))
	m := testkit.ReadJSON(t, conn, 2*time.Second, "the AAPL quote")
	if m["type"] != "quote" || m["symbol"] != "AAPL" || m["bid"] != 2.0 {
		t.Fatalf("got %v, want the AAPL quote with bid 2 (and nothing for MSFT)", m)
	}
	e.hub.PublishTrade(market.Trade{Symbol: "AAPL", Price: 2.5, Size: 10, Timestamp: ts0})
	m = testkit.ReadJSON(t, conn, 2*time.Second, "the AAPL trade")
	if m["type"] != "trade" || m["price"] != 2.5 {
		t.Fatalf("got %v, want the AAPL trade", m)
	}
}

func TestCheckpoint4_9_Unsubscribe(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL","MSFT"]}`)
	e.waitSubscribed(t, "MSFT", 1)
	testkit.Send(t, conn, `{"action":"unsubscribe","symbols":["AAPL"]}`)
	testkit.Eventually(t, 3*time.Second, func() bool { _, ok := e.hub.Stats().Subscriptions["AAPL"]; return !ok },
		"after the browser unsubscribed from AAPL, the hub still has it subscribed")
	e.hub.PublishQuote(q("AAPL", 1))
	e.hub.PublishQuote(q("MSFT", 2))
	if m := testkit.ReadJSON(t, conn, 2*time.Second, "the MSFT quote"); m["symbol"] != "MSFT" {
		t.Fatalf("got %v, want only the MSFT quote", m)
	}
}

func TestCheckpoint4_9_BadMessagesGetAnError(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	for _, bad := range []string{
		`not json`,
		`{"action":"buy","symbols":["AAPL"]}`,
		`{"action":"subscribe","symbols":["123"]}`,
	} {
		testkit.Send(t, conn, bad)
		m := testkit.ReadJSON(t, conn, 2*time.Second, fmt.Sprintf("an error reply to %s", bad))
		if m["type"] != "error" || m["code"] != "bad_request" {
			t.Fatalf("sent %s, got %v; want {\"type\":\"error\",\"code\":\"bad_request\",...}", bad, m)
		}
		if msg, _ := m["message"].(string); msg == "" {
			t.Fatalf("the error reply %v should have a non-empty message", m)
		}
	}
	// The connection stays usable after bad input.
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL"]}`)
	e.waitSubscribed(t, "AAPL", 1)
}

func TestCheckpoint4_9_SymbolLimitError(t *testing.T) {
	e := setup(t, hub.Config{MaxSymbols: 2}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL","MSFT","GOOG"]}`)
	m := testkit.ReadJSON(t, conn, 2*time.Second, "a symbol_limit error")
	if m["type"] != "error" || m["code"] != "symbol_limit" {
		t.Fatalf("got %v, want {\"type\":\"error\",\"code\":\"symbol_limit\",...}", m)
	}
}

func TestCheckpoint4_9_DisconnectCleansUp(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	a := testkit.DialWS(t, e.srv.URL, "/")
	b := testkit.DialWS(t, e.srv.URL, "/")
	testkit.Send(t, a, `{"action":"subscribe","symbols":["AAPL"]}`)
	testkit.Send(t, b, `{"action":"subscribe","symbols":["AAPL","MSFT"]}`)
	e.waitSubscribed(t, "AAPL", 2)

	a.Close()
	e.waitClients(t, 1)
	e.waitSubscribed(t, "AAPL", 1)
	b.Close()
	e.waitClients(t, 0)
	if s := e.hub.Stats().Subscriptions; len(s) != 0 {
		t.Fatalf("after every browser left, the hub still has subscriptions %v", s)
	}
}

func TestCheckpoint4_9_ManyBrowsers(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	const n = 20
	for i := 0; i < n; i++ {
		conn := testkit.DialWS(t, e.srv.URL, "/")
		testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL"]}`)
		defer func(i int) {
			for k := 1; k <= 50; k++ {
				m := testkit.ReadJSON(t, conn, 3*time.Second, fmt.Sprintf("browser %d's quote #%d", i, k))
				if m["bid"] != float64(k) {
					t.Fatalf("browser %d got %v, want bid %d (in order)", i, m, k)
				}
			}
		}(i)
	}
	e.waitSubscribed(t, "AAPL", n)
	for k := 1; k <= 50; k++ {
		e.hub.PublishQuote(q("AAPL", float64(k)))
	}
}

func TestCheckpoint4_9_HubShutdownClosesBrowsers(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	e.waitClients(t, 1)
	e.stopHub(t)
	testkit.ExpectClosed(t, conn, 3*time.Second,
		"When the hub closes a client's Messages channel (shutdown or eviction), the session should close the WebSocket.")
}

func TestCheckpoint4_9_BrowserLeavingStopsBothLoops(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{})
	for i := 0; i < 5; i++ {
		conn := testkit.DialWS(t, e.srv.URL, "/")
		testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL"]}`)
		e.waitSubscribed(t, "AAPL", 1)
		conn.Close()
		e.waitClients(t, 0)
	}
	e.srv.CloseClientConnections()
	e.stopHub(t)
	testkit.NoLeaks(t, 3*time.Second, testkit.Module+"/internal/")
}
