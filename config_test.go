package main

import (
	"encoding/json"
	"io"
	"sync"
	"testing"
)

func validConfig() Config {
	return Config{Universe: "static10", Limit: 10, RefreshPeriod: 21600, SortMetric: "alt_rank", Order: "asc"}
}

func TestConfigCanonicalIDStableAndDefaults(t *testing.T) {
	a := validConfig()
	b := Config{Order: "asc", SortMetric: "alt_rank", RefreshPeriod: 21600, Limit: 10, Universe: "static10", Filters: map[string]Range{}}
	ca, err := a.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := b.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if ca.ID() != cb.ID() {
		t.Fatalf("IDs differ: %s %s", ca.ID(), cb.ID())
	}
	if len(ca.ID()) != 22 {
		t.Fatalf("opaque ID length=%d", len(ca.ID()))
	}
}

func TestPublicBaseURLRequiresOrigin(t *testing.T) {
	for _, raw := range []string{
		"https://pairs.example/base",
		"https://pairs.example/?x=1",
		"https://pairs.example?",
		"https://pairs.example#",
		"https://user:pass@pairs.example",
	} {
		if _, err := validatePublicBaseURL(raw); err == nil {
			t.Fatalf("accepted non-origin %q", raw)
		}
	}
	got, err := validatePublicBaseURL("https://pairs.example/")
	if err != nil || got != "https://pairs.example" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestStrictConfigJSONRejectsUnknownAndInvalid(t *testing.T) {
	unsupported := validConfig()
	unsupported.SortMetric = "contributors_active"
	if _, err := unsupported.Canonical(); err == nil {
		t.Fatal("accepted metric that is not present in LunarCrush coins/list/v1")
	}
	supported := validConfig()
	supported.SortMetric = "social_volume_24h"
	if _, err := supported.Canonical(); err != nil {
		t.Fatalf("rejected documented list metric: %v", err)
	}
	cases := []string{
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc","oops":1}`,
		`{"universe":"static10","limit":0,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"bogus","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":1,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"code","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"sideways"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc","filters":{"alt_rank":{"min":4,"max":2}}}`,
		`{"universe":"static10","universe":"hyperliquid","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc","filters":{"alt_rank":{"min":1,"min":2}}}`,
		`{"universe":"static10","limit":10,"Limit":1,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc","filters":{"alt_rank":{"min":1,"Min":2}}}`,
		`{"universe":"static10","univerſe":"hyperliquid","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"}`,
		`{"universe":"static10","limit":10,"refresh_period":21600,"sort_metric":"alt_rank","order":"asc"} trailing`,
	}
	for _, raw := range cases {
		if _, err := DecodeConfigJSON(json.NewDecoder(stringsReader(raw))); err == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func stringsReader(s string) *stringReader { return &stringReader{s: s} }

type stringReader struct{ s string }

func (r *stringReader) Read(p []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}
	n := copy(p, r.s)
	r.s = r.s[n:]
	return n, nil
}

func TestConcurrentConfigCreationDeduplicates(t *testing.T) {
	s := testStore(t)
	const n = 24
	ids := make(chan string, n)
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := s.PutConfig(validConfig())
			if err != nil {
				errs <- err
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	var first string
	count := 0
	for id := range ids {
		count++
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("got %q and %q", first, id)
		}
	}
	if count != n {
		t.Fatalf("count=%d", count)
	}
	if got := s.ConfigCount(); got != 1 {
		t.Fatalf("rows=%d", got)
	}
}
