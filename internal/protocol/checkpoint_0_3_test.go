//go:build all || m0 || m0_3

// Checkpoint 0.3: encoding server messages

package protocol_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/wwelden/datadash/internal/market"
	"github.com/wwelden/datadash/internal/protocol"
)

func decode(t *testing.T, fn string, b []byte, err error) map[string]any {
	t.Helper()
	if err != nil {
		t.Fatalf("%s returned error: %v", fn, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s returned %q, which is not a JSON object: %v", fn, b, err)
	}
	return m
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func checkKeys(t *testing.T, fn string, m map[string]any, want ...string) {
	t.Helper()
	sort.Strings(want)
	if got := keys(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s produced JSON keys %v, want exactly %v.\n"+
			"JSON key names come from struct tags like `json:\"bidSize\"`.", fn, got, want)
	}
}

func checkTS(t *testing.T, fn string, m map[string]any, want time.Time) {
	t.Helper()
	s, ok := m["ts"].(string)
	if !ok {
		t.Fatalf("%s: \"ts\" should be a string, got %#v", fn, m["ts"])
	}
	got, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("%s: \"ts\" = %q is not RFC 3339: %v", fn, s, err)
	}
	if !got.Equal(want) {
		t.Fatalf("%s: \"ts\" = %q, want the instant %s", fn, s, want.UTC().Format(time.RFC3339Nano))
	}
	if s[len(s)-1] != 'Z' {
		t.Fatalf("%s: \"ts\" = %q should be in UTC (ending in Z). Hint: time.Time has a UTC() method.", fn, s)
	}
}

func TestCheckpoint0_3_EncodeQuote(t *testing.T) {
	ts := time.Date(2026, 11, 6, 18, 30, 0, 123_000_000, time.UTC)
	b, err := protocol.EncodeQuote(market.Quote{
		Symbol: "AAPL", BidPrice: 187.10, BidSize: 2, AskPrice: 187.12, AskSize: 3, Timestamp: ts,
	})
	m := decode(t, "EncodeQuote", b, err)
	checkKeys(t, "EncodeQuote", m, "type", "symbol", "bid", "bidSize", "ask", "askSize", "ts")
	want := map[string]any{"type": "quote", "symbol": "AAPL", "bid": 187.10, "bidSize": 2.0, "ask": 187.12, "askSize": 3.0}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("EncodeQuote: %q = %#v, want %#v", k, m[k], v)
		}
	}
	checkTS(t, "EncodeQuote", m, ts)
}

func TestCheckpoint0_3_EncodeTrade(t *testing.T) {
	ts := time.Date(2026, 11, 6, 18, 30, 0, 125_000_000, time.UTC)
	b, err := protocol.EncodeTrade(market.Trade{Symbol: "AAPL", Price: 187.11, Size: 100, Timestamp: ts})
	m := decode(t, "EncodeTrade", b, err)
	checkKeys(t, "EncodeTrade", m, "type", "symbol", "price", "size", "ts")
	want := map[string]any{"type": "trade", "symbol": "AAPL", "price": 187.11, "size": 100.0}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("EncodeTrade: %q = %#v, want %#v", k, m[k], v)
		}
	}
	checkTS(t, "EncodeTrade", m, ts)
}

func TestCheckpoint0_3_TimestampsAreUTC(t *testing.T) {
	eastern := time.FixedZone("EST", -5*60*60)
	ts := time.Date(2026, 11, 6, 13, 30, 0, 0, eastern) // 18:30 UTC
	b, err := protocol.EncodeTrade(market.Trade{Symbol: "AAPL", Price: 1, Size: 1, Timestamp: ts})
	m := decode(t, "EncodeTrade", b, err)
	checkTS(t, "EncodeTrade (with a non-UTC timestamp)", m, ts)
}

func TestCheckpoint0_3_EncodeStatus(t *testing.T) {
	if protocol.StatusConnected != "connected" || protocol.StatusReconnecting != "reconnecting" {
		t.Fatalf("StatusConnected/StatusReconnecting = %q/%q, want \"connected\"/\"reconnecting\"",
			protocol.StatusConnected, protocol.StatusReconnecting)
	}
	b, err := protocol.EncodeStatus(protocol.StatusReconnecting)
	m := decode(t, "EncodeStatus", b, err)
	checkKeys(t, "EncodeStatus", m, "type", "upstream")
	if m["type"] != "status" || m["upstream"] != "reconnecting" {
		t.Fatalf("EncodeStatus(\"reconnecting\") = %s, want {\"type\":\"status\",\"upstream\":\"reconnecting\"}", b)
	}
}

func TestCheckpoint0_3_EncodeError(t *testing.T) {
	if protocol.CodeBadRequest != "bad_request" || protocol.CodeSymbolLimit != "symbol_limit" {
		t.Fatalf("CodeBadRequest/CodeSymbolLimit = %q/%q, want \"bad_request\"/\"symbol_limit\"",
			protocol.CodeBadRequest, protocol.CodeSymbolLimit)
	}
	b, err := protocol.EncodeError(protocol.CodeSymbolLimit, "Too many symbols")
	m := decode(t, "EncodeError", b, err)
	checkKeys(t, "EncodeError", m, "type", "code", "message")
	if m["type"] != "error" || m["code"] != "symbol_limit" || m["message"] != "Too many symbols" {
		t.Fatalf("EncodeError = %s, want {\"type\":\"error\",\"code\":\"symbol_limit\",\"message\":\"Too many symbols\"}", b)
	}
}

func TestCheckpoint0_3_SpecialCharactersAreEscaped(t *testing.T) {
	msg := `bad "symbol" <x>`
	b, err := protocol.EncodeError(protocol.CodeBadRequest, msg)
	m := decode(t, "EncodeError", b, err)
	if m["message"] != msg {
		t.Fatalf("EncodeError message round-tripped as %#v, want %#v.\n"+
			"Build JSON with encoding/json, not by gluing strings together.", m["message"], msg)
	}
}
