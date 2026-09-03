package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLunarClientBearerAuthAndRetry(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("auth=%q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("limit") != "1000" {
			t.Errorf("limit=%q", r.URL.Query().Get("limit"))
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(429)
			return
		}
		io.WriteString(w, `{"data":[{"symbol":"BTC","alt_rank":1,"galaxy_score":70,"sentiment":80,"interactions_24h":10,"social_volume_24h":7,"volume_24h":20,"market_cap":30}]}`)
	}))
	defer ts.Close()
	c := NewLunarClient(ts.URL, "secret", ClientOptions{Timeout: time.Second, MaxBytes: 4096, Retries: 2, Backoff: time.Millisecond})
	got, err := c.Assets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || len(got) != 1 || got[0].Symbol != "BTC" || got[0].Metrics["social_volume_24h"] != 7 {
		t.Fatalf("calls=%d got=%v", calls.Load(), got)
	}
}
func TestClientTimeoutAndResponseLimit(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(100 * time.Millisecond)
		}
		io.WriteString(w, strings.Repeat("x", 200))
	}))
	defer ts.Close()
	c := NewLunarClient(ts.URL+"/slow", "x", ClientOptions{Timeout: 10 * time.Millisecond, MaxBytes: 100, Retries: 0})
	if _, err := c.Assets(context.Background()); err == nil {
		t.Fatal("timeout accepted")
	}
	c = NewLunarClient(ts.URL, "x", ClientOptions{Timeout: time.Second, MaxBytes: 100, Retries: 0})
	if _, err := c.Assets(context.Background()); err == nil {
		t.Fatal("oversized response accepted")
	}
}
func TestClientsRejectMalformedSuccessfulResponses(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":[]}`, `{"data":[{}]}`, `{"data":[{"symbol":"   "}]}`} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
		_, err := NewLunarClient(ts.URL, "key", ClientOptions{Timeout: time.Second, MaxBytes: 4096}).Assets(context.Background())
		ts.Close()
		if err == nil {
			t.Fatalf("LunarCrush accepted %s", body)
		}
	}
	for _, body := range []string{
		`[{"universe":[{"name":"BTC"}]},{}]`,
		`[{"universe":[]},[]]`,
		`[{"universe":[{}]},[{}]]`,
		`[{"universe":[{"name":"   "}]},[{}]]`,
		`[{"universe":[{"name":"BTC"}]}]`,
		`[{"universe":[{"name":"BTC"},{"name":"ETH"}]},[{}]]`,
	} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
		_, err := NewHyperClient(ts.URL, ClientOptions{Timeout: time.Second, MaxBytes: 4096}).Markets(context.Background())
		ts.Close()
		if err == nil {
			t.Fatalf("Hyperliquid accepted %s", body)
		}
	}
}

func TestHyperliquidActiveMarketsAndRetry(t *testing.T) {
	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method=%s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", r.Header.Get("Content-Type"))
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload["type"] != "metaAndAssetCtxs" {
			t.Errorf("invalid request payload: payload=%v err=%v", payload, err)
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		io.WriteString(w, `[{"universe":[{"name":"BTC"},{"name":"ETH","isDelisted":true}]},[{"funding":"0.0001"},{"funding":"0.0002"}]]`)
	}))
	defer ts.Close()
	c := NewHyperClient(ts.URL, ClientOptions{Timeout: time.Second, MaxBytes: 4096, Retries: 1, Backoff: time.Millisecond})
	got, err := c.Markets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Active || got[1].Active {
		t.Fatalf("got=%v", got)
	}
}
