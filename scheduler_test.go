package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type fakeCollector struct {
	assets  []Asset
	markets []HyperMarket
	err     error
	ac, mc  int
}

func (f *fakeCollector) Assets(context.Context) ([]Asset, error) { f.ac++; return f.assets, f.err }
func (f *fakeCollector) Markets(context.Context) ([]HyperMarket, error) {
	f.mc++
	return f.markets, f.err
}
func TestSchedulerUsesOneSharedSnapshotAndKeepsLastGood(t *testing.T) {
	s := testStore(t)
	a, _ := s.PutConfig(validConfig())
	c := validConfig()
	c.Limit = 2
	b, _ := s.PutConfig(c)
	f := &fakeCollector{assets: []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}}, markets: []HyperMarket{{Name: "BTC", Active: true}}}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.ac != 1 || f.mc != 1 {
		t.Fatalf("fetches=%d,%d", f.ac, f.mc)
	}
	f.err = errors.New("upstream unavailable")
	if err := sch.Refresh(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	state, err := s.GetRefreshState(a)
	if err != nil || state.Error != "upstream refresh failed" || state.AttemptedAt.IsZero() {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	for _, id := range []string{a, b} {
		r, err := s.GetResult(id)
		if err != nil || len(r.Pairs) != 1 {
			t.Fatalf("id=%s result=%v err=%v", id, r, err)
		}
	}
}

func TestStaticUniverseStillRequiresAnActiveHyperliquidMarket(t *testing.T) {
	s := testStore(t)
	id, _ := s.PutConfig(validConfig())
	f := &fakeCollector{
		assets: []Asset{
			{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}},
			{Symbol: "ETH", Metrics: map[string]float64{"alt_rank": 2}},
		},
		markets: []HyperMarket{{Name: "BTC", Active: true}, {Name: "ETH", Active: false}},
	}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := s.GetResult(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pairs) != 1 || result.Pairs[0] != "BTC/USDC:USDC" {
		t.Fatalf("inactive static market was published: %v", result.Pairs)
	}
}

func TestNewConfigUsesTheLatestSharedSnapshotImmediately(t *testing.T) {
	s := testStore(t)
	f := &fakeCollector{
		assets:  []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}},
		markets: []HyperMarket{{Name: "BTC", Active: true}},
	}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	h := NewApp(s, sch).Handler()
	body, _ := json.Marshal(validConfig())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var made ConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &made); err != nil {
		t.Fatal(err)
	}
	result, err := s.GetResult(made.ID)
	if err != nil {
		t.Fatalf("new config was not evaluated from shared snapshot: %v", err)
	}
	if !reflect.DeepEqual(result.Pairs, []string{"BTC/USDC:USDC"}) {
		t.Fatalf("pairs=%v", result.Pairs)
	}
	if f.ac != 1 || f.mc != 1 {
		t.Fatalf("config creation made another upstream request: assets=%d markets=%d", f.ac, f.mc)
	}
}

func TestEmptySelectionDoesNotTriggerUpstreamRefresh(t *testing.T) {
	s := testStore(t)
	f := &fakeCollector{
		assets:  []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}},
		markets: []HyperMarket{{Name: "BTC", Active: true}},
	}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	config := validConfig()
	minimum := 999.0
	config.Filters = map[string]Range{"alt_rank": {Min: &minimum}}
	body, _ := json.Marshal(config)
	rr := httptest.NewRecorder()
	NewApp(s, sch).Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if queued := len(sch.trigger); queued != 0 {
		t.Fatalf("empty selection queued %d upstream refresh", queued)
	}
	if f.ac != 1 || f.mc != 1 {
		t.Fatalf("upstream calls=%d,%d", f.ac, f.mc)
	}
}

func TestQueuedTriggerEvaluatesCurrentSnapshotWithoutRefetch(t *testing.T) {
	s := testStore(t)
	f := &fakeCollector{
		assets:  []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}},
		markets: []HyperMarket{{Name: "BTC", Active: true}},
	}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	id, err := s.PutConfig(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.handleTrigger(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.ac != 1 || f.mc != 1 {
		t.Fatalf("queued trigger refetched upstream: %d,%d", f.ac, f.mc)
	}
	if _, err := s.GetResult(id); err != nil {
		t.Fatalf("queued config not evaluated: %v", err)
	}
}

func TestSchedulerLoadsPersistedSharedSnapshotAfterRestart(t *testing.T) {
	s := testStore(t)
	at := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	assets := []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}}
	markets := []HyperMarket{{Name: "BTC", Active: true}}
	if err := s.SaveSnapshot("lunarcrush", assets, at); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnapshot("hyperliquid", markets, at); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{}
	sch := NewScheduler(s, f, f, time.Hour)
	id, _ := s.PutConfig(validConfig())
	if err := sch.EvaluateConfig(id, validConfig()); err != nil {
		t.Fatalf("persisted snapshot was not loaded: %v", err)
	}
	result, err := s.GetResult(id)
	if err != nil {
		t.Fatal(err)
	}
	if !result.UpdatedAt.Equal(at) {
		t.Fatalf("result timestamp=%s want=%s", result.UpdatedAt, at)
	}
	if f.ac != 0 || f.mc != 0 {
		t.Fatalf("persisted snapshot caused upstream calls: %d,%d", f.ac, f.mc)
	}
}

func TestConfigurationCreatedAfterFailedRefreshSeesGlobalFailure(t *testing.T) {
	s := testStore(t)
	snapshotAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	if err := s.SaveSharedSnapshots(
		[]Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}},
		[]HyperMarket{{Name: "BTC", Active: true}}, snapshotAt,
	); err != nil {
		t.Fatal(err)
	}
	f := &fakeCollector{err: errors.New("upstream unavailable")}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err == nil {
		t.Fatal("expected failed refresh")
	}
	id, _ := s.PutConfig(validConfig())
	if err := sch.EvaluateConfig(id, validConfig()); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	NewApp(s, sch).Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/status/"+id, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var status Status
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Error != "upstream refresh failed" || status.AttemptedAt == nil || !status.AttemptedAt.After(snapshotAt) {
		t.Fatalf("global failure missing from status: %+v", status)
	}
}

func TestOldSnapshotEvaluationDoesNotHideNewerUpstreamFailure(t *testing.T) {
	s := testStore(t)
	snapshotAt := time.Now().UTC().Add(-4 * time.Hour).Truncate(time.Second)
	failureAt := snapshotAt.Add(2 * time.Hour)
	assets := []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}}
	markets := []HyperMarket{{Name: "BTC", Active: true}}
	if err := s.SaveSnapshot("lunarcrush", assets, snapshotAt); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnapshot("hyperliquid", markets, snapshotAt); err != nil {
		t.Fatal(err)
	}
	id, _ := s.PutConfig(validConfig())
	if err := s.SetRefreshState(id, "upstream refresh failed", failureAt); err != nil {
		t.Fatal(err)
	}
	sch := NewScheduler(s, &fakeCollector{}, &fakeCollector{}, time.Hour)
	if err := sch.EvaluateConfig(id, validConfig()); err != nil {
		t.Fatal(err)
	}
	state, err := s.GetRefreshState(id)
	if err != nil {
		t.Fatal(err)
	}
	if state.Error != "upstream refresh failed" || !state.AttemptedAt.Equal(failureAt) {
		t.Fatalf("newer failure was hidden: %+v", state)
	}
}

func TestConfigEvaluationPreservesSharedSnapshotAge(t *testing.T) {
	s := testStore(t)
	f := &fakeCollector{
		assets:  []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}},
		markets: []HyperMarket{{Name: "BTC", Active: true}},
	}
	sch := NewScheduler(s, f, f, time.Hour)
	if err := sch.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	sch.mu.Lock()
	sch.snapshot.at = old
	sch.mu.Unlock()
	id, _ := s.PutConfig(validConfig())
	if err := sch.EvaluateConfig(id, validConfig()); err != nil {
		t.Fatal(err)
	}
	result, err := s.GetResult(id)
	if err != nil {
		t.Fatal(err)
	}
	if !result.UpdatedAt.Equal(old) {
		t.Fatalf("result timestamp=%s want snapshot timestamp=%s", result.UpdatedAt, old)
	}
}
