//go:build all || m6 || m6_1

// Checkpoint 6.1: heartbeats and deadlines

package client_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/wwelden/datadash/internal/client"
	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint6_1_ServerSendsPings(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{
		PingInterval: 50 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
		WriteWait:    time.Second,
	})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	var pings atomic.Int32
	conn.SetPingHandler(func(data string) error {
		pings.Add(1)
		// reply like a browser does
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})
	// A browser is always reading; reading is what processes pings.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(500 * time.Millisecond)
	if n := pings.Load(); n < 4 {
		t.Fatalf("with PingInterval 50ms the server sent only %d pings in 500ms", n)
	}
	// We've been answering pings, so we must still be connected after
	// more than twice PongWait.
	if c := e.hub.Stats().Clients; c != 1 {
		t.Fatalf("a client that answers pings was disconnected (Stats().Clients = %d). "+
			"Each pong should push the read deadline forward by PongWait.", c)
	}
}

func TestCheckpoint6_1_SilentClientIsDropped(t *testing.T) {
	e := setup(t, hub.Config{}, client.Config{
		PingInterval: 50 * time.Millisecond,
		PongWait:     200 * time.Millisecond,
		WriteWait:    time.Second,
	})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	e.waitClients(t, 1)
	// We never read, so we never answer pings: like a laptop that went to sleep.
	start := time.Now()
	e.waitClients(t, 0)
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("a silent client was only dropped after %v; it should go shortly after PongWait (200ms)", el)
	}
	_ = conn
}

func TestCheckpoint6_1_StuckSocketIsDropped(t *testing.T) {
	e := setup(t, hub.Config{QueueSize: 8}, client.Config{
		PingInterval: time.Minute,
		PongWait:     2 * time.Minute, // so only the write deadline can save us
		WriteWait:    200 * time.Millisecond,
	})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["AAPL"]}`)
	e.waitSubscribed(t, "AAPL", 1)

	// Flood a client that never reads until the network buffers fill and the
	// server's writes block. The write deadline (or eviction) must free it.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			e.hub.PublishQuote(q("AAPL", float64(i)))
			if i%64 == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}()
	testkit.Eventually(t, 10*time.Second, func() bool { return e.hub.Stats().Clients == 0 },
		"a client whose socket stopped accepting data was never dropped")

	// The hub has let go of it, but are the session's goroutines stuck in a
	// write that will never finish? (We still aren't reading.)
	testkit.NoLeaks(t, 3*time.Second, testkit.Module+"/internal/client.")
	_ = conn
}

func TestCheckpoint6_1_DefaultsAreSane(t *testing.T) {
	// Zero Config means "use defaults", and a normal client must not get
	// dropped quickly.
	e := setup(t, hub.Config{}, client.Config{})
	conn := testkit.DialWS(t, e.srv.URL, "/")
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	if c := e.hub.Stats().Clients; c != 1 {
		t.Fatalf("with the default Config a healthy client was dropped within 300ms (Clients = %d)", c)
	}
}
