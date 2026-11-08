//go:build all || m7 || m7_2

// Checkpoint 7.2: the dashboard page

package server_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCheckpoint7_2_DashboardPage(t *testing.T) {
	r := startServer(t)
	resp, err := http.Get(r.url + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / returned %s, want 200", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET / has Content-Type %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	page := string(body)
	for _, want := range []string{"<table", "IEX", "/ws"} {
		if !strings.Contains(page, want) {
			t.Errorf("the dashboard page should contain %q", want)
		}
	}
}

func TestCheckpoint7_2_UnknownPathIs404(t *testing.T) {
	r := startServer(t)
	resp, err := http.Get(r.url + "/definitely-not-here")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /definitely-not-here returned %s, want 404", resp.Status)
	}
}
