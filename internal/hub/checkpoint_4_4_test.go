//go:build all || m4 || m4_4

// Checkpoint 4.4: the symbol cap

package hub_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wwelden/datadash/internal/hub"
)

func TestCheckpoint4_4_RejectsOverCap(t *testing.T) {
	h := startHub(t, 3, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")
	subscribe(t, h, "B", b, "GOOG")

	err := trySubscribe(t, h, b, "TSLA")
	if !errors.Is(err, hub.ErrSymbolLimit) {
		t.Fatalf("a 4th distinct symbol with MaxSymbols 3 returned %v, want hub.ErrSymbolLimit", err)
	}
	s := h.stats(t)
	if _, ok := s.Subscriptions["TSLA"]; ok || len(s.Subscriptions) != 3 {
		t.Fatalf("a rejected Subscribe still changed the hub: Stats().Subscriptions = %v", s.Subscriptions)
	}
	h.up.waitActive(t, "AAPL", "GOOG", "MSFT")
}

func TestCheckpoint4_4_ExistingSymbolsDontCount(t *testing.T) {
	h := startHub(t, 3, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT", "GOOG")
	// At the cap, but these symbols are already open upstream: no new cost.
	if err := trySubscribe(t, h, b, "AAPL", "GOOG"); err != nil {
		t.Fatalf("subscribing to symbols that are already subscribed should work at the cap, got %v", err)
	}
	if got := h.stats(t).Subscriptions["AAPL"]; got != 2 {
		t.Fatalf("Stats().Subscriptions[\"AAPL\"] = %d, want 2", got)
	}
}

func TestCheckpoint4_4_AllOrNothing(t *testing.T) {
	h := startHub(t, 3, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")

	// One existing symbol plus two new ones would make 4 distinct symbols.
	err := trySubscribe(t, h, b, "AAPL", "GOOG", "TSLA")
	if !errors.Is(err, hub.ErrSymbolLimit) {
		t.Fatalf("got %v, want hub.ErrSymbolLimit", err)
	}
	s := h.stats(t)
	if s.Subscriptions["AAPL"] != 1 || len(s.Subscriptions) != 2 {
		t.Fatalf("a rejected request must change nothing, not even the symbols that fit. Stats().Subscriptions = %v, want map[AAPL:1 MSFT:1]", s.Subscriptions)
	}
}

func TestCheckpoint4_4_RoomFreesUp(t *testing.T) {
	h := startHub(t, 2, 0)
	a, b := h.register(t), h.register(t)
	subscribe(t, h, "A", a, "AAPL", "MSFT")
	if err := trySubscribe(t, h, b, "GOOG"); !errors.Is(err, hub.ErrSymbolLimit) {
		t.Fatalf("got %v, want hub.ErrSymbolLimit", err)
	}
	unsubscribe(t, h, "A", a, "MSFT")
	if err := trySubscribe(t, h, b, "GOOG"); err != nil {
		t.Fatalf("after A released MSFT there's room, but Subscribe returned %v", err)
	}
	h.up.waitActive(t, "AAPL", "GOOG")
}

func TestCheckpoint4_4_DefaultCapIs30(t *testing.T) {
	h := startHub(t, 0, 0) // 0 means use the default
	a := h.register(t)
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for i := 0; i < 30; i++ {
		sym := fmt.Sprintf("%c%c", letters[i%26], letters[i/26])
		if err := trySubscribe(t, h, a, sym); err != nil {
			t.Fatalf("symbol #%d (%s) was rejected with %v; with MaxSymbols 0 the cap should default to 30", i+1, sym, err)
		}
	}
	if err := trySubscribe(t, h, a, "ZZZ"); !errors.Is(err, hub.ErrSymbolLimit) {
		t.Fatalf("the 31st symbol returned %v, want hub.ErrSymbolLimit", err)
	}
}
