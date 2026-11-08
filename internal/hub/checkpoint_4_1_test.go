//go:build all || m4 || m4_1

// Checkpoint 4.1: hub lifecycle (register, unregister, stats, shutdown)

package hub_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/testkit"
)

func TestCheckpoint4_1_RegisterCountsClients(t *testing.T) {
	h := startHub(t, 0, 0)
	if s := h.stats(t); s.Clients != 0 {
		t.Fatalf("a new hub should report 0 clients, Stats().Clients = %d", s.Clients)
	}
	a := h.register(t)
	b := h.register(t)
	h.register(t)
	if a == b {
		t.Fatal("two Register calls returned the same *Client")
	}
	if s := h.stats(t); s.Clients != 3 {
		t.Fatalf("after 3 Register calls, Stats().Clients = %d, want 3", s.Clients)
	}
}

func TestCheckpoint4_1_UnregisterClosesMessages(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	b := h.register(t)
	testkit.Within(t, 2*time.Second, "Hub.Unregister", func() { h.Unregister(a) })
	expectClosed(t, "the unregistered client", a, 2*time.Second)
	if s := h.stats(t); s.Clients != 1 {
		t.Fatalf("after unregistering 1 of 2 clients, Stats().Clients = %d, want 1", s.Clients)
	}
	select {
	case _, ok := <-b.Messages():
		if !ok {
			t.Fatal("unregistering client A closed client B's channel")
		}
	default:
	}
}

func TestCheckpoint4_1_UnregisterTwiceIsSafe(t *testing.T) {
	h := startHub(t, 0, 0)
	a := h.register(t)
	testkit.Within(t, 2*time.Second, "the first Unregister", func() { h.Unregister(a) })
	// Closing a channel twice panics. The hub must remember it already removed a.
	testkit.Within(t, 2*time.Second, "a second Unregister of the same client", func() { h.Unregister(a) })
	if s := h.stats(t); s.Clients != 0 {
		t.Fatalf("Stats().Clients = %d, want 0", s.Clients)
	}
}

func TestCheckpoint4_1_StatsWithManyConcurrentRegisters(t *testing.T) {
	h := startHub(t, 0, 0)
	clients := make(chan *hub.Client, 50)
	testkit.Within(t, 5*time.Second, "50 concurrent Register calls", func() {
		done := make(chan struct{})
		for i := 0; i < 50; i++ {
			go func() {
				c, err := h.Register()
				if err != nil {
					t.Errorf("Register: %v", err)
				}
				clients <- c
				done <- struct{}{}
			}()
		}
		for i := 0; i < 50; i++ {
			<-done
		}
	})
	if s := h.stats(t); s.Clients != 50 {
		t.Fatalf("after 50 concurrent Register calls, Stats().Clients = %d", s.Clients)
	}
	close(clients)
	for c := range clients {
		go h.Unregister(c)
	}
	testkit.Eventually(t, 2*time.Second, func() bool { return h.Stats().Clients == 0 },
		"after unregistering all 50 clients concurrently, Stats().Clients never reached 0")
}

func TestCheckpoint4_1_ShutdownClosesEverything(t *testing.T) {
	up := newFakeUpstream(30, 0)
	h := hub.New(hub.Config{Upstream: up})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Run(ctx)
	}()
	th := &testHub{Hub: h, up: up, cancel: cancel, done: done}
	a := th.register(t)
	b := th.register(t)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Hub.Run did not return within 2s after its context was cancelled")
	}
	expectClosed(t, "client A after shutdown", a, 2*time.Second)
	expectClosed(t, "client B after shutdown", b, 2*time.Second)

	var err error
	testkit.Within(t, time.Second, "Register after the hub stopped", func() { _, err = h.Register() })
	if !errors.Is(err, hub.ErrClosed) {
		t.Fatalf("Register after shutdown returned %v, want hub.ErrClosed", err)
	}
	testkit.Within(t, time.Second, "Unregister after the hub stopped", func() { h.Unregister(a) })
	var s hub.Stats
	testkit.Within(t, time.Second, "Stats after the hub stopped", func() { s = h.Stats() })
	if s.Clients != 0 {
		t.Fatalf("Stats after shutdown = %+v, want the zero Stats", s)
	}
	testkit.NoLeaks(t, 2*time.Second, testkit.Module+"/internal/")
}
