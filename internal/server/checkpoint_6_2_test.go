//go:build all || m6 || m6_2

// Checkpoint 6.2: the whole server, health check and graceful shutdown

package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint6_2_EndToEndWithFakeFeed(t *testing.T) {
	r := startServer(t)
	conn := testkit.DialWS(t, r.url, "/ws")
	testkit.Send(t, conn, `{"action":"subscribe","symbols":["FAKEPACA"]}`)
	for i := 0; i < 3; i++ {
		m := testkit.ReadJSON(t, conn, 3*time.Second, "FAKEPACA data from the fake feed")
		if m["symbol"] != "FAKEPACA" || (m["type"] != "quote" && m["type"] != "trade") {
			t.Fatalf("got %v, want FAKEPACA quotes or trades", m)
		}
	}
}

func TestCheckpoint6_2_Healthz(t *testing.T) {
	r := startServer(t)
	conn := testkit.DialWS(t, r.url, "/ws")
	_ = conn
	testkit.Eventually(t, 3*time.Second, func() bool {
		resp, err := http.Get(r.url + "/healthz")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		var body struct {
			Status  string `json:"status"`
			Clients int    `json:"clients"`
		}
		if json.NewDecoder(resp.Body).Decode(&body) != nil {
			return false
		}
		return body.Status == "ok" && body.Clients == 1
	}, "GET /healthz should return 200 with JSON {\"status\":\"ok\",\"clients\":1} while one browser is connected")
}

func TestCheckpoint6_2_GracefulShutdown(t *testing.T) {
	r := startServer(t)
	a := testkit.DialWS(t, r.url, "/ws")
	b := testkit.DialWS(t, r.url, "/ws")
	testkit.Send(t, a, `{"action":"subscribe","symbols":["FAKEPACA"]}`)
	testkit.ReadJSON(t, a, 3*time.Second, "data before shutdown")

	r.cancel()
	select {
	case err := <-r.done:
		if err != nil {
			t.Fatalf("server.Run returned %v after a normal shutdown; want nil", err)
		}
		r.done <- nil // let the cleanup see it too
	case <-time.After(5 * time.Second):
		t.Fatal("server.Run did not return within 5s of cancelling its context")
	}
	testkit.ExpectClosed(t, a, 2*time.Second, "Shutdown should close every browser connection.")
	testkit.ExpectClosed(t, b, 2*time.Second, "Shutdown should close every browser connection.")
	if _, err := http.Get(r.url + "/healthz"); err == nil {
		t.Fatal("the server still answers HTTP after Run returned")
	}
	testkit.NoLeaks(t, 3*time.Second, testkit.Module+"/internal/")
}
