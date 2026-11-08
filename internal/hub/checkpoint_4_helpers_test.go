//go:build all || m4 || m4_1 || m4_2 || m4_3 || m4_4 || m4_5 || m4_6 || m4_7

// Shared helpers for the hub checkpoints (4.1 to 4.7).

package hub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/hub"
	"github.com/wwelden/datadash/internal/testkit"
)

// fakeUpstream records what the hub asks of Alpaca and checks the rules:
// no overlapping calls, no double-subscribing, never over the cap.
type fakeUpstream struct {
	delay    time.Duration
	max      int
	inFlight atomic.Int32

	mu       sync.Mutex
	active   map[string]bool
	subs     map[string]int // how many times each symbol was subscribed
	unsubs   map[string]int
	problems []string
}

func newFakeUpstream(max int, delay time.Duration) *fakeUpstream {
	return &fakeUpstream{
		delay: delay, max: max,
		active: map[string]bool{}, subs: map[string]int{}, unsubs: map[string]int{},
	}
}

func (f *fakeUpstream) problem(format string, args ...any) {
	f.problems = append(f.problems, fmt.Sprintf(format, args...))
}

func (f *fakeUpstream) enter(op string, symbols []string) func() {
	if f.inFlight.Add(1) > 1 {
		f.mu.Lock()
		f.problem("%s(%v) was called while another upstream call was still running. "+
			"The SDK forbids concurrent subscription changes: one goroutine must make them in order.", op, symbols)
		f.mu.Unlock()
	}
	if f.delay > 0 {
		time.Sleep(f.delay) // pretend to wait for Alpaca
	}
	return func() { f.inFlight.Add(-1) }
}

func (f *fakeUpstream) Subscribe(symbols []string) error {
	defer f.enter("Subscribe", symbols)()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(symbols) == 0 {
		f.problem("Subscribe was called with no symbols")
	}
	for _, s := range symbols {
		if f.active[s] {
			f.problem("Subscribe(%q) was sent upstream, but %q was already subscribed upstream", s, s)
		}
		f.active[s] = true
		f.subs[s]++
	}
	if len(f.active) > f.max {
		f.problem("upstream ended up with %d symbols, over the cap of %d", len(f.active), f.max)
	}
	return nil
}

func (f *fakeUpstream) Unsubscribe(symbols []string) error {
	defer f.enter("Unsubscribe", symbols)()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(symbols) == 0 {
		f.problem("Unsubscribe was called with no symbols")
	}
	for _, s := range symbols {
		if !f.active[s] {
			f.problem("Unsubscribe(%q) was sent upstream, but %q wasn't subscribed upstream", s, s)
		}
		delete(f.active, s)
		f.unsubs[s]++
	}
	return nil
}

func (f *fakeUpstream) Active() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.active))
	for s := range f.active {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func (f *fakeUpstream) Unsubs(symbol string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.unsubs[symbol]
}

func (f *fakeUpstream) Subs(symbol string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subs[symbol]
}

func (f *fakeUpstream) check(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, p := range f.problems {
		if i == 5 {
			t.Errorf("...and %d more upstream problems", len(f.problems)-5)
			break
		}
		t.Error("upstream: " + p)
	}
}

func (f *fakeUpstream) waitActive(t *testing.T, want ...string) {
	t.Helper()
	sort.Strings(want)
	testkit.Eventually(t, 2*time.Second, func() bool {
		return strings.Join(f.Active(), ",") == strings.Join(want, ",")
	}, "upstream subscriptions never became %v (they are %v)", want, lazy(f.Active))
}

type lazy func() []string

func (l lazy) String() string { return fmt.Sprint(l()) }

type testHub struct {
	*hub.Hub
	up     *fakeUpstream
	cancel context.CancelFunc
	done   chan struct{}
}

// startHub creates a hub, runs it, and stops it when the test ends.
func startHub(t *testing.T, maxSymbols, queueSize int) *testHub {
	t.Helper()
	capFor := maxSymbols
	if capFor == 0 {
		capFor = 30
	}
	up := newFakeUpstream(capFor, time.Millisecond)
	var h *hub.Hub
	testkit.Within(t, time.Second, "hub.New", func() {
		h = hub.New(hub.Config{Upstream: up, MaxSymbols: maxSymbols, QueueSize: queueSize})
	})
	if h == nil {
		t.Fatal("hub.New returned nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Run(ctx)
	}()
	th := &testHub{Hub: h, up: up, cancel: cancel, done: done}
	t.Cleanup(func() {
		th.stop(t)
		up.check(t)
	})
	return th
}

func (th *testHub) stop(t *testing.T) {
	t.Helper()
	th.cancel()
	select {
	case <-th.done:
	case <-time.After(2 * time.Second):
		t.Error("Hub.Run did not return within 2s after its context was cancelled. Does the loop select on ctx.Done()?")
	}
}

func (th *testHub) register(t *testing.T) *hub.Client {
	t.Helper()
	var c *hub.Client
	var err error
	testkit.Within(t, 2*time.Second, "Hub.Register", func() { c, err = th.Register() })
	if err != nil {
		t.Fatalf("Register returned error %v", err)
	}
	if c == nil {
		t.Fatal("Register returned a nil client")
	}
	if c.Messages() == nil {
		t.Fatal("client.Messages() returned a nil channel")
	}
	return c
}

func (th *testHub) stats(t *testing.T) hub.Stats {
	t.Helper()
	var s hub.Stats
	testkit.Within(t, 2*time.Second, "Hub.Stats", func() { s = th.Stats() })
	return s
}

// next receives one message for c and decodes it.
func next(t *testing.T, who string, c *hub.Client) map[string]any {
	t.Helper()
	select {
	case b, ok := <-c.Messages():
		if !ok {
			t.Fatalf("%s's Messages channel was closed while a message was expected", who)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s received %q, which is not a JSON object: %v", who, b, err)
		}
		return m
	case <-time.After(2 * time.Second):
		t.Fatalf("%s waited 2s for a message but none arrived", who)
	}
	return nil
}

// expectClosed waits for c's channel to be closed, discarding messages.
// It returns how many messages were discarded.
func expectClosed(t *testing.T, who string, c *hub.Client, d time.Duration) int {
	t.Helper()
	n := 0
	timeout := time.After(d)
	for {
		select {
		case _, ok := <-c.Messages():
			if !ok {
				return n
			}
			n++
		case <-timeout:
			t.Fatalf("%s's Messages channel was never closed (waited %v)", who, d)
		}
	}
}
