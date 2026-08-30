package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPairlistLastKnownGoodAndEmptyRejected(t *testing.T) {
	s := testStore(t)
	id, err := s.PutConfig(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveResult(id, []string{"BTC/USDC:USDC"}, map[string]string{"ETH": "filtered"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveResult(id, nil, nil, time.Now().UTC()); err == nil {
		t.Fatal("empty result accepted")
	}
	r, err := s.GetResult(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Pairs, []string{"BTC/USDC:USDC"}) {
		t.Fatalf("pairs=%v", r.Pairs)
	}
}

func TestPairlistEndpointShapeAndMissingResult(t *testing.T) {
	s := testStore(t)
	h := NewApp(s, nil).Handler()
	id, _ := s.PutConfig(validConfig())
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/pairlist/"+id, nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body)
	}
	if err := s.SaveResult(id, []string{"BTC/USDC:USDC"}, nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/pairlist/"+id, nil))
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body)
	}
	var got map[string]any
	json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got) != 2 || got["refresh_period"] != float64(21600) {
		t.Fatalf("shape=%v", got)
	}
}

func TestCreateConfigAndStatusSanitized(t *testing.T) {
	s := testStore(t)
	h := NewApp(s, nil).Handler()
	body, _ := json.Marshal(validConfig())
	req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
	req.Host = "pairs.example"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body)
	}
	var made ConfigResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &made); err != nil {
		t.Fatal(err)
	}
	if made.ID == "" || made.PairlistURL != "http://pairs.example/api/pairlist/"+made.ID {
		t.Fatalf("response=%+v", made)
	}
	if !bytes.Contains([]byte(made.FreqtradeConfig), []byte(`"method":"RemotePairList"`)) {
		t.Fatalf("snippet=%s", made.FreqtradeConfig)
	}
	var snippet map[string]any
	if err := json.Unmarshal([]byte(made.FreqtradeConfig), &snippet); err != nil {
		t.Fatalf("invalid snippet: %v", err)
	}
	for key, want := range map[string]any{
		"method":                   "RemotePairList",
		"mode":                     "whitelist",
		"processing_mode":          "filter",
		"number_assets":            float64(10),
		"refresh_period":           float64(21600),
		"keep_pairlist_on_failure": true,
		"read_timeout":             float64(10),
		"save_to_file":             "user_data/last_lunarcrush_pairlist.json",
	} {
		if snippet[key] != want {
			t.Fatalf("snippet[%q]=%v want %v; full=%v", key, snippet[key], want, snippet)
		}
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/status/"+made.ID, nil))
	if rr.Code != 200 || bytes.Contains(rr.Body.Bytes(), []byte("token")) {
		t.Fatalf("status body=%s", rr.Body)
	}
}

func TestResultPublicationAndStateClearAreAtomic(t *testing.T) {
	s := testStore(t)
	id, _ := s.PutConfig(validConfig())
	snapshotAt := time.Now().UTC().Truncate(time.Second)
	if err := s.SetRefreshState(id, "upstream refresh failed", snapshotAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_state_clear BEFORE UPDATE ON refresh_state WHEN NEW.error='' BEGIN SELECT RAISE(FAIL, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.PublishResult(id, []string{"BTC/USDC:USDC"}, nil, snapshotAt, snapshotAt); err == nil {
		t.Fatal("injected state-clear failure succeeded")
	}
	if _, err := s.GetResult(id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("result committed despite state failure: %v", err)
	}
	state, err := s.GetRefreshState(id)
	if err != nil || state.Error != "upstream refresh failed" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestSharedSnapshotWriteIsAtomic(t *testing.T) {
	s := testStore(t)
	oldAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	oldAssets := []Asset{{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}}
	oldMarkets := []HyperMarket{{Name: "BTC", Active: true}}
	if err := s.SaveSharedSnapshots(oldAssets, oldMarkets, oldAt); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER fail_hyper_snapshot BEFORE INSERT ON snapshots WHEN NEW.kind='hyperliquid' BEGIN SELECT RAISE(FAIL, 'injected'); END`); err != nil {
		t.Fatal(err)
	}
	newAt := oldAt.Add(time.Hour)
	if err := s.SaveSharedSnapshots([]Asset{{Symbol: "ETH"}}, []HyperMarket{{Name: "ETH", Active: true}}, newAt); err == nil {
		t.Fatal("injected second snapshot failure succeeded")
	}
	var gotAssets []Asset
	var gotMarkets []HyperMarket
	assetsAt, err := s.LoadSnapshot("lunarcrush", &gotAssets)
	if err != nil {
		t.Fatal(err)
	}
	marketsAt, err := s.LoadSnapshot("hyperliquid", &gotMarkets)
	if err != nil {
		t.Fatal(err)
	}
	if !assetsAt.Equal(oldAt) || !marketsAt.Equal(oldAt) || gotAssets[0].Symbol != "BTC" || gotMarkets[0].Name != "BTC" {
		t.Fatalf("partial snapshot commit: assets=%v at=%s markets=%v at=%s", gotAssets, assetsAt, gotMarkets, marketsAt)
	}
}

func TestStoreBoundsConfigurationsAndAuditRows(t *testing.T) {
	s := testStore(t)
	s.SetLimits(1, 2)
	first := validConfig()
	if _, err := s.PutConfig(first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutConfig(first); err != nil {
		t.Fatalf("deduplicated configuration rejected at cap: %v", err)
	}
	second := validConfig()
	second.Limit = 2
	if _, err := s.PutConfig(second); !errors.Is(err, ErrConfigLimit) {
		t.Fatalf("err=%v want ErrConfigLimit", err)
	}
	body, _ := json.Marshal(second)
	rr := httptest.NewRecorder()
	NewApp(s, nil).Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body)))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	for i := 0; i < 3; i++ {
		s.Audit("", "test", "bounded")
	}
	var auditRows int
	if err := s.db.QueryRow(`SELECT count(*) FROM audit`).Scan(&auditRows); err != nil || auditRows != 2 {
		t.Fatalf("audit rows=%d err=%v", auditRows, err)
	}
}

func TestInactiveConfigurationsExpireWithoutBreakingActiveIDs(t *testing.T) {
	s := testStore(t)
	s.SetLimits(1, 10)
	s.SetConfigTTL(time.Hour)
	firstID, err := s.PutConfig(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`UPDATE config_activity SET last_accessed_at=? WHERE config_id=?`, old, firstID); err != nil {
		t.Fatal(err)
	}
	second := validConfig()
	second.Limit = 2
	if _, err := s.PutConfig(second); err != nil {
		t.Fatalf("inactive configuration was not pruned: %v", err)
	}
	if _, _, err := s.GetConfig(firstID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired configuration still exists: %v", err)
	}
}

func TestConfigCreationRateLimit(t *testing.T) {
	s := testStore(t)
	app := NewApp(s, nil)
	app.SetCreateRateLimit(1, time.Hour)
	body, _ := json.Marshal(validConfig())
	for attempt := 1; attempt <= 2; attempt++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(body))
		req.RemoteAddr = "198.51.100.7:1234"
		app.Handler().ServeHTTP(rr, req)
		want := http.StatusCreated
		if attempt == 2 {
			want = http.StatusTooManyRequests
		}
		if rr.Code != want {
			t.Fatalf("attempt=%d status=%d body=%s", attempt, rr.Code, rr.Body.String())
		}
	}
}

func TestSecurityHeadersMethodsAndBodyLimit(t *testing.T) {
	h := NewApp(testStore(t), nil).Handler()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/config", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rr.Code)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" || rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("headers=%v", rr.Header())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(make([]byte, MaxRequestBody+1))))
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rr.Code)
	}
}
