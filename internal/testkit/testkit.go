// Package testkit holds helpers shared by the checkpoint tests.
//
// You don't need to read or change anything in here. It exists so the
// checkpoint tests can give you clear failure messages instead of hanging.
package testkit

import (
	"encoding/json"
	"errors"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Module is this project's module path.
const Module = "github.com/wwelden/datadash"

// Within runs fn in its own goroutine and fails the test if fn has not
// returned after d. fn must not call t.Fatal (t.Error is fine).
func Within(t testing.TB, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not finish within %v.\n"+
			"It is probably stuck: a deadlock, a send nobody receives, or a channel nobody closes.", what, d)
	}
}

// Eventually polls cond until it returns true, failing the test with the
// given message if that doesn't happen within d. Each call to cond is also
// guarded, so a call into your code that never returns fails instead of
// freezing the test run.
func Eventually(t testing.TB, d time.Duration, cond func() bool, format string, args ...any) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		result := make(chan bool, 1)
		go func() { result <- cond() }()
		select {
		case ok := <-result:
			if ok {
				return
			}
		case <-time.After(time.Until(deadline) + 50*time.Millisecond):
			t.Fatalf(format+"\n(While checking, a call into your code never returned. Is something blocked?)", args...)
		}
		if time.Now().After(deadline) {
			t.Fatalf(format, args...)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// NoLeaks fails the test if, after waiting up to wait, any goroutine is
// still running a function whose name starts with one of prefixes
// (for example "main." or Module+"/internal/").
func NoLeaks(t testing.TB, wait time.Duration, prefixes ...string) {
	t.Helper()
	deadline := time.Now().Add(wait)
	var leaked []string
	for {
		leaked = leakedGoroutines(prefixes)
		if len(leaked) == 0 {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(leaked) > 3 {
		leaked = leaked[:3]
	}
	t.Fatalf("goroutine leak: %d goroutine(s) from your code are still running after everything should have stopped.\n"+
		"A goroutine leak usually means a goroutine is blocked on a channel nobody will use again,\n"+
		"or a loop that never checks ctx.Done(). Here's where they're stuck:\n\n%s",
		len(leakedGoroutines(prefixes)), strings.Join(leaked, "\n\n"))
}

func leakedGoroutines(prefixes []string) []string {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, len(buf)*2)
	}
	blocks := strings.Split(string(buf), "\n\n")
	var leaked []string
	for i, block := range blocks {
		if i == 0 { // the goroutine calling NoLeaks
			continue
		}
		if strings.Contains(block, "testing.tRunner") || strings.Contains(block, "testing.(*T).Run") {
			continue
		}
		if blockIsUsers(block, prefixes) {
			leaked = append(leaked, block)
		}
	}
	return leaked
}

func blockIsUsers(block string, prefixes []string) bool {
	for _, line := range strings.Split(block, "\n") {
		if line == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "goroutine ") {
			continue
		}
		line = strings.TrimPrefix(line, "created by ")
		if strings.Contains(line, ".Test") || strings.Contains(line, "/internal/testkit.") {
			continue
		}
		for _, p := range prefixes {
			if strings.HasPrefix(line, p) {
				return true
			}
		}
	}
	return false
}

// DialWS opens a WebSocket to an httptest server URL (http://...) plus path.
// The connection is closed automatically when the test ends.
func DialWS(t testing.TB, serverURL, path string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(serverURL, "http") + path
	d := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
	conn, resp, err := d.Dial(u, nil)
	if err != nil {
		status := "no HTTP response"
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("could not open a WebSocket to %s: %v (%s).\n"+
			"Check that the handler calls Upgrade and doesn't return early.", u, err, status)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// Send writes a text message, failing the test on error.
func Send(t testing.TB, conn *websocket.Conn, msg string) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		t.Fatalf("sending %q to the server failed: %v (did the server close the connection?)", msg, err)
	}
}

// ReadText reads one message and returns its type and payload. It fails
// the test if nothing arrives within d.
func ReadText(t testing.TB, conn *websocket.Conn, d time.Duration, waitingFor string) (int, string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(d))
	mt, p, err := conn.ReadMessage()
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatalf("waited %v for %s, but no message arrived", d, waitingFor)
		}
		t.Fatalf("while waiting for %s, the connection failed: %v", waitingFor, err)
	}
	return mt, string(p)
}

// ReadJSON reads one message and decodes it as a JSON object.
func ReadJSON(t testing.TB, conn *websocket.Conn, d time.Duration, waitingFor string) map[string]any {
	t.Helper()
	_, p := ReadText(t, conn, d, waitingFor)
	return DecodeObject(t, []byte(p))
}

// DecodeObject decodes a JSON object, failing the test if it isn't one.
func DecodeObject(t testing.TB, p []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(p, &m); err != nil {
		t.Fatalf("expected a JSON object, got %q (%v)", p, err)
	}
	return m
}

// ExpectClosed reads (and discards) messages until the connection fails.
// It fails the test if the connection is still open after d.
func ExpectClosed(t testing.TB, conn *websocket.Conn, d time.Duration, hint string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(d))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			t.Fatalf("expected the server to close the connection within %v, but it stayed open. %s", d, hint)
		}
		return
	}
}
