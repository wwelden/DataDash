//go:build all || m0 || m0_2

// Checkpoint 0.2: parsing browser messages

package protocol_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wwelden/datadash/internal/protocol"
)

func TestCheckpoint0_2_ActionConstants(t *testing.T) {
	if protocol.ActionSubscribe != "subscribe" {
		t.Errorf("ActionSubscribe = %q, want %q", protocol.ActionSubscribe, "subscribe")
	}
	if protocol.ActionUnsubscribe != "unsubscribe" {
		t.Errorf("ActionUnsubscribe = %q, want %q", protocol.ActionUnsubscribe, "unsubscribe")
	}
}

func TestCheckpoint0_2_ParseValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  protocol.ClientMessage
	}{
		{
			name:  "subscribe two symbols",
			input: `{"action":"subscribe","symbols":["AAPL","MSFT"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"AAPL", "MSFT"}},
		},
		{
			name:  "unsubscribe one symbol",
			input: `{"action":"unsubscribe","symbols":["MSFT"]}`,
			want:  protocol.ClientMessage{Action: "unsubscribe", Symbols: []string{"MSFT"}},
		},
		{
			name:  "lowercase symbols are uppercased",
			input: `{"action":"subscribe","symbols":["aapl","Msft"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"AAPL", "MSFT"}},
		},
		{
			name:  "spaces around symbols are trimmed",
			input: `{"action":"subscribe","symbols":["  AAPL ","\tTSLA"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"AAPL", "TSLA"}},
		},
		{
			name:  "duplicates removed, first-seen order kept",
			input: `{"action":"subscribe","symbols":["MSFT","AAPL","msft","AAPL","GOOG"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"MSFT", "AAPL", "GOOG"}},
		},
		{
			name:  "dotted class shares are allowed",
			input: `{"action":"subscribe","symbols":["BRK.B"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"BRK.B"}},
		},
		{
			name:  "the test stream symbol",
			input: `{"action":"subscribe","symbols":["FAKEPACA"]}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"FAKEPACA"}},
		},
		{
			name:  "unknown extra fields are ignored",
			input: `{"action":"subscribe","symbols":["AAPL"],"extra":123}`,
			want:  protocol.ClientMessage{Action: "subscribe", Symbols: []string{"AAPL"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := protocol.ParseClientMessage([]byte(tc.input))
			if err != nil {
				t.Fatalf("ParseClientMessage(%s) returned error %v, want no error", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseClientMessage(%s)\n got: %+v\nwant: %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestCheckpoint0_2_ParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
		// if set, err.Error() must contain this text
		mention string
	}{
		{"not JSON", `hello`, protocol.ErrInvalidJSON, ""},
		{"truncated JSON", `{"action":"subscribe"`, protocol.ErrInvalidJSON, ""},
		{"JSON array instead of object", `["AAPL"]`, protocol.ErrInvalidJSON, ""},
		{"symbols is not a list", `{"action":"subscribe","symbols":"AAPL"}`, protocol.ErrInvalidJSON, ""},
		{"empty input", ``, protocol.ErrInvalidJSON, ""},
		{"unknown action", `{"action":"buy","symbols":["AAPL"]}`, protocol.ErrUnknownAction, ""},
		{"missing action", `{"symbols":["AAPL"]}`, protocol.ErrUnknownAction, ""},
		{"action is case-sensitive", `{"action":"SUBSCRIBE","symbols":["AAPL"]}`, protocol.ErrUnknownAction, ""},
		{"missing symbols", `{"action":"subscribe"}`, protocol.ErrNoSymbols, ""},
		{"empty symbols", `{"action":"subscribe","symbols":[]}`, protocol.ErrNoSymbols, ""},
		{"empty symbol string", `{"action":"subscribe","symbols":["AAPL",""]}`, protocol.ErrInvalidSymbol, ""},
		{"only spaces", `{"action":"subscribe","symbols":["   "]}`, protocol.ErrInvalidSymbol, ""},
		{"digits not allowed", `{"action":"subscribe","symbols":["AAPL1"]}`, protocol.ErrInvalidSymbol, "AAPL1"},
		{"wildcard not allowed", `{"action":"subscribe","symbols":["*"]}`, protocol.ErrInvalidSymbol, "*"},
		{"slash not allowed", `{"action":"subscribe","symbols":["BTC/USD"]}`, protocol.ErrInvalidSymbol, "BTC/USD"},
		{"too long (11 chars)", `{"action":"subscribe","symbols":["ABCDEFGHIJK"]}`, protocol.ErrInvalidSymbol, "ABCDEFGHIJK"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := protocol.ParseClientMessage([]byte(tc.input))
			if err == nil {
				t.Fatalf("ParseClientMessage(%s) returned no error, want %v", tc.input, tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseClientMessage(%s) returned %q, which errors.Is does not match to %v.\n"+
					"Return the sentinel error itself, or wrap it with fmt.Errorf(\"...: %%w\", ...).",
					tc.input, err, tc.wantErr)
			}
			if tc.mention != "" && !strings.Contains(strings.ToUpper(err.Error()), tc.mention) {
				t.Fatalf("error %q should mention the bad symbol %q so the user knows which one was wrong", err, tc.mention)
			}
		})
	}
}

func TestCheckpoint0_2_ExactlyTenCharsAllowed(t *testing.T) {
	got, err := protocol.ParseClientMessage([]byte(`{"action":"subscribe","symbols":["ABCDEFGHIJ"]}`))
	if err != nil {
		t.Fatalf("a 10-character symbol should be allowed, got error %v", err)
	}
	if len(got.Symbols) != 1 || got.Symbols[0] != "ABCDEFGHIJ" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckpoint0_2_SentinelErrorsAreDistinct(t *testing.T) {
	errs := map[string]error{
		"ErrInvalidJSON":   protocol.ErrInvalidJSON,
		"ErrUnknownAction": protocol.ErrUnknownAction,
		"ErrNoSymbols":     protocol.ErrNoSymbols,
		"ErrInvalidSymbol": protocol.ErrInvalidSymbol,
	}
	for name, e := range errs {
		if e == nil {
			t.Errorf("%s is nil; create it with errors.New", name)
		}
		for other, o := range errs {
			if name != other && e != nil && errors.Is(e, o) {
				t.Errorf("%s and %s should be different errors", name, other)
			}
		}
	}
}
