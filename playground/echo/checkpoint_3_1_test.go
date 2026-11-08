//go:build all || m3 || m3_1

// Checkpoint 3.1: WebSocket echo server

package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wwelden/datadash/internal/testkit"
)

func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(wsEndpoint))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckpoint3_1_EchoesText(t *testing.T) {
	srv := echoServer(t)
	conn := testkit.DialWS(t, srv.URL, "/")
	testkit.Send(t, conn, "hello")
	mt, got := testkit.ReadText(t, conn, 2*time.Second, "the echo of \"hello\"")
	if got != "hello" {
		t.Fatalf("sent \"hello\", got back %q", got)
	}
	if mt != websocket.TextMessage {
		t.Fatalf("sent a text message but got back message type %d", mt)
	}
}

func TestCheckpoint3_1_EchoesBinaryAsBinary(t *testing.T) {
	srv := echoServer(t)
	conn := testkit.DialWS(t, srv.URL, "/")
	payload := []byte{0, 1, 2, 255}
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatal(err)
	}
	mt, got := testkit.ReadText(t, conn, 2*time.Second, "the echo of a binary message")
	if mt != websocket.BinaryMessage || got != string(payload) {
		t.Fatalf("sent binary %v, got back type %d payload %v. Echo the same message type you received.", payload, mt, []byte(got))
	}
}

func TestCheckpoint3_1_KeepsOrder(t *testing.T) {
	srv := echoServer(t)
	conn := testkit.DialWS(t, srv.URL, "/")
	for i := 0; i < 50; i++ {
		testkit.Send(t, conn, fmt.Sprintf("msg-%d", i))
	}
	for i := 0; i < 50; i++ {
		want := fmt.Sprintf("msg-%d", i)
		if _, got := testkit.ReadText(t, conn, 2*time.Second, want); got != want {
			t.Fatalf("echo #%d was %q, want %q", i, got, want)
		}
	}
}

// Many browsers connecting at the same moment. With -race this also catches
// handlers that write to shared variables (like the upgrader) on every request.
func TestCheckpoint3_1_ManyClientsAtOnce(t *testing.T) {
	srv := echoServer(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			u := "ws" + srv.URL[len("http"):]
			conn, _, err := websocket.DefaultDialer.Dial(u, nil)
			if err != nil {
				errs <- fmt.Errorf("client %d could not connect: %v", i, err)
				return
			}
			defer conn.Close()
			want := fmt.Sprintf("client-%d", i)
			conn.WriteMessage(websocket.TextMessage, []byte(want))
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, got, err := conn.ReadMessage()
			if err != nil || string(got) != want {
				errs <- fmt.Errorf("client %d sent %q, got %q (err %v): clients' messages got mixed up or lost", i, want, got, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestCheckpoint3_1_PlainHTTPGets400(t *testing.T) {
	srv := echoServer(t)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("a plain (non-WebSocket) GET request broke the server: %v.\n"+
			"The handler probably panicked: after Upgrade fails, log the error and return.", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("a plain GET got status %d, want 400 (the upgrader writes it for you when Upgrade fails)", resp.StatusCode)
	}
}

func TestCheckpoint3_1_ServerClosesItsSide(t *testing.T) {
	srv := echoServer(t)
	conn := testkit.DialWS(t, srv.URL, "/")
	testkit.Send(t, conn, "ping")
	testkit.ReadText(t, conn, 2*time.Second, "the echo of \"ping\"")

	// Hang up our sending side. The server's read loop sees end-of-file and
	// returns. If the handler closes the connection when it returns, our next
	// read fails right away instead of timing out.
	tcp, ok := conn.UnderlyingConn().(*net.TCPConn)
	if !ok {
		t.Skip("not a TCP connection")
	}
	tcp.CloseWrite()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("after the client hung up, the server never closed its end of the connection.\n" +
			"Once Upgrade succeeds, defer closing the connection so it's released when the handler returns.")
	}
}

func TestCheckpoint3_1_HandlersFinish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(wsEndpoint))
	for i := 0; i < 5; i++ {
		conn := testkit.DialWS(t, srv.URL, "/")
		testkit.Send(t, conn, "bye")
		testkit.ReadText(t, conn, 2*time.Second, "an echo")
		conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		conn.Close()
	}
	srv.CloseClientConnections()
	srv.Close()
	testkit.NoLeaks(t, 2*time.Second, "main.")
}
