//go:build all || m3 || m3_2

// Checkpoint 3.2: chat room

package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wwelden/datadash/internal/testkit"
)

type chatRoom struct {
	room   *Room
	srv    *httptest.Server
	cancel context.CancelFunc
	done   chan struct{}
}

func startRoom(t *testing.T) *chatRoom {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var room *Room
	testkit.Within(t, time.Second, "NewRoom()", func() { room = NewRoom() })
	if room == nil {
		t.Fatal("NewRoom returned nil")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(ctx)
	}()
	srv := httptest.NewServer(room)
	c := &chatRoom{room: room, srv: srv, cancel: cancel, done: done}
	t.Cleanup(func() {
		cancel()
		srv.CloseClientConnections()
		srv.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Room.Run did not return within 2s after its context was cancelled")
		}
	})
	return c
}

func (c *chatRoom) waitForClients(t *testing.T, n int) {
	t.Helper()
	testkit.Eventually(t, 2*time.Second, func() bool { return c.room.ClientCount() == n },
		"waited for ClientCount() to become %d", n)
}

func expectMsg(t *testing.T, who string, conn *websocket.Conn, want string) {
	t.Helper()
	if _, got := testkit.ReadText(t, conn, 2*time.Second, fmt.Sprintf("%s to receive %q", who, want)); got != want {
		t.Fatalf("%s received %q, want %q", who, got, want)
	}
}

func TestCheckpoint3_2_CountsClients(t *testing.T) {
	c := startRoom(t)
	if n := c.room.ClientCount(); n != 0 {
		t.Fatalf("a new room should have 0 clients, ClientCount() = %d", n)
	}
	a := testkit.DialWS(t, c.srv.URL, "/")
	testkit.DialWS(t, c.srv.URL, "/")
	c.waitForClients(t, 2)
	a.Close()
	c.waitForClients(t, 1)
}

func TestCheckpoint3_2_BroadcastsToEveryone(t *testing.T) {
	c := startRoom(t)
	a := testkit.DialWS(t, c.srv.URL, "/")
	b := testkit.DialWS(t, c.srv.URL, "/")
	d := testkit.DialWS(t, c.srv.URL, "/")
	c.waitForClients(t, 3)

	testkit.Send(t, a, "hello from A")
	expectMsg(t, "A (the sender)", a, "hello from A")
	expectMsg(t, "B", b, "hello from A")
	expectMsg(t, "C", d, "hello from A")

	testkit.Send(t, b, "hi, B here")
	expectMsg(t, "A", a, "hi, B here")
	expectMsg(t, "B (the sender)", b, "hi, B here")
	expectMsg(t, "C", d, "hi, B here")
}

func TestCheckpoint3_2_LeavingClientIsRemoved(t *testing.T) {
	c := startRoom(t)
	a := testkit.DialWS(t, c.srv.URL, "/")
	b := testkit.DialWS(t, c.srv.URL, "/")
	gone := testkit.DialWS(t, c.srv.URL, "/")
	c.waitForClients(t, 3)

	gone.Close()
	c.waitForClients(t, 2)

	testkit.Send(t, a, "still here?")
	expectMsg(t, "A", a, "still here?")
	expectMsg(t, "B", b, "still here?")
}

func TestCheckpoint3_2_KeepsEachSendersOrder(t *testing.T) {
	c := startRoom(t)
	a := testkit.DialWS(t, c.srv.URL, "/")
	b := testkit.DialWS(t, c.srv.URL, "/")
	c.waitForClients(t, 2)
	for i := 0; i < 100; i++ {
		testkit.Send(t, a, fmt.Sprintf("line %d", i))
	}
	for i := 0; i < 100; i++ {
		expectMsg(t, "B", b, fmt.Sprintf("line %d", i))
	}
}

func TestCheckpoint3_2_ManyTalkersAtOnce(t *testing.T) {
	c := startRoom(t)
	const clients, each = 5, 20
	conns := make([]*websocket.Conn, clients)
	for i := range conns {
		conns[i] = testkit.DialWS(t, c.srv.URL, "/")
	}
	c.waitForClients(t, clients)
	for i, conn := range conns {
		go func(i int, conn *websocket.Conn) {
			for j := 0; j < each; j++ {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%d:%d", i, j)))
			}
		}(i, conn)
	}
	// every client should get all clients*each messages
	for i, conn := range conns {
		for k := 0; k < clients*each; k++ {
			testkit.ReadText(t, conn, 3*time.Second, fmt.Sprintf("client %d's message #%d (of %d)", i, k+1, clients*each))
		}
	}
}

func TestCheckpoint3_2_ShutdownClosesEverything(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	room := NewRoom()
	done := make(chan struct{})
	go func() {
		defer close(done)
		room.Run(ctx)
	}()
	srv := httptest.NewServer(room)
	defer srv.Close()

	a := testkit.DialWS(t, srv.URL, "/")
	b := testkit.DialWS(t, srv.URL, "/")
	testkit.Eventually(t, 2*time.Second, func() bool { return room.ClientCount() == 2 }, "waited for 2 clients")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Room.Run did not return within 2s after its context was cancelled. Does the loop select on ctx.Done()?")
	}
	testkit.ExpectClosed(t, a, 2*time.Second, "When the room shuts down it should close every client connection.")
	testkit.ExpectClosed(t, b, 2*time.Second, "When the room shuts down it should close every client connection.")
	srv.CloseClientConnections()
	testkit.NoLeaks(t, 2*time.Second, "main.")
}
