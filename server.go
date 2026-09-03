package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const MaxRequestBody int64 = 16384

//go:embed web/*
var webFiles embed.FS

type App struct {
	store      *Store
	scheduler  *Scheduler
	publicBase string
	rateMu     sync.Mutex
	rate       map[string]rateWindow
	rateLimit  int
	ratePeriod time.Duration
}

type rateWindow struct {
	started time.Time
	count   int
}
type ConfigResponse struct {
	ID              string `json:"id"`
	PairlistURL     string `json:"pairlist_url"`
	StatusURL       string `json:"status_url"`
	FreqtradeConfig string `json:"freqtrade_config"`
}

func NewApp(s *Store, sch *Scheduler) *App {
	return &App{store: s, scheduler: sch, rate: map[string]rateWindow{}, rateLimit: 20, ratePeriod: time.Minute}
}
func (a *App) SetPublicBase(v string) { a.publicBase = strings.TrimRight(v, "/") }
func (a *App) SetCreateRateLimit(limit int, period time.Duration) {
	if limit > 0 && period > 0 {
		a.rateLimit, a.ratePeriod = limit, period
	}
}

func (a *App) allowConfigCreate(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	now := time.Now()
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	entry, exists := a.rate[host]
	if exists && now.Sub(entry.started) >= a.ratePeriod {
		delete(a.rate, host)
		exists = false
	}
	if !exists && len(a.rate) >= 1024 {
		for key, candidate := range a.rate {
			if now.Sub(candidate.started) >= a.ratePeriod {
				delete(a.rate, key)
			}
		}
		if len(a.rate) >= 1024 {
			return false
		}
	}
	if !exists {
		entry = rateWindow{started: now}
	}
	if entry.count >= a.rateLimit {
		return false
	}
	entry.count++
	a.rate[host] = entry
	return true
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFiles, "web")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			writeError(w, 404, "not found")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		b, _ := webFiles.ReadFile("web/index.html")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("POST /api/config", a.createConfig)
	mux.HandleFunc("GET /api/pairlist/{id}", a.pairlist)
	mux.HandleFunc("GET /api/status/{id}", a.status)
	return security(mux)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.store.db.PingContext(r.Context()); err != nil {
		writeError(w, 503, "not ready")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ready"})
}
func (a *App) createConfig(w http.ResponseWriter, r *http.Request) {
	if !a.allowConfigCreate(r) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "configuration creation rate limit exceeded")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBody+1))
	if err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if int64(len(body)) > MaxRequestBody {
		writeError(w, 413, "request body too large")
		return
	}
	c, err := DecodeConfigJSON(json.NewDecoder(strings.NewReader(string(body))))
	if err != nil {
		writeError(w, 400, "invalid configuration")
		return
	}
	id, err := a.store.PutConfig(c)
	if errors.Is(err, ErrConfigLimit) {
		writeError(w, http.StatusTooManyRequests, "configuration capacity reached")
		return
	}
	if err != nil {
		writeError(w, 500, "configuration could not be stored")
		return
	}
	if a.scheduler != nil {
		if err := a.scheduler.EvaluateConfig(id, c); err != nil {
			if errors.Is(err, ErrSnapshotUnavailable) {
				a.scheduler.Trigger()
			}
		}
	}
	base := a.publicBase
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	pair := base + "/api/pairlist/" + id
	status := base + "/api/status/" + id
	snippetBytes, _ := json.Marshal(struct {
		Method        string `json:"method"`
		Mode          string `json:"mode"`
		Processing    string `json:"processing_mode"`
		PairlistURL   string `json:"pairlist_url"`
		NumberAssets  int    `json:"number_assets"`
		RefreshPeriod int    `json:"refresh_period"`
		KeepOnFailure bool   `json:"keep_pairlist_on_failure"`
		ReadTimeout   int    `json:"read_timeout"`
		SaveToFile    string `json:"save_to_file"`
	}{"RemotePairList", "whitelist", "filter", pair, c.Limit, c.RefreshPeriod, true, 10, "user_data/last_lunarcrush_pairlist.json"})
	writeJSON(w, http.StatusCreated, ConfigResponse{id, pair, status, string(snippetBytes)})
}
func validID(id string) bool {
	if len(id) != 22 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
func (a *App) pairlist(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, 404, "configuration not found")
		return
	}
	c, _, err := a.store.GetConfig(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "configuration not found")
		return
	}
	if err != nil {
		writeError(w, 500, "request failed")
		return
	}
	if err := a.store.TouchConfig(id); err != nil {
		writeError(w, 500, "request failed")
		return
	}
	res, err := a.store.GetResult(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 503, "no valid pairlist is available yet")
		return
	}
	if err != nil {
		writeError(w, 500, "request failed")
		return
	}
	writeJSON(w, 200, struct {
		Pairs   []string `json:"pairs"`
		Refresh int      `json:"refresh_period"`
	}{res.Pairs, c.RefreshPeriod})
}
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, 404, "configuration not found")
		return
	}
	c, created, err := a.store.GetConfig(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "configuration not found")
		return
	}
	if err != nil {
		writeError(w, 500, "request failed")
		return
	}
	if err := a.store.TouchConfig(id); err != nil {
		writeError(w, 500, "request failed")
		return
	}
	st := Status{Config: c, CreatedAt: created, Pairs: []string{}, Exclusions: map[string]string{}}
	res, err := a.store.GetResult(id)
	if err == nil {
		st.UpdatedAt = &res.UpdatedAt
		st.Fresh = time.Since(res.UpdatedAt) <= time.Duration(c.RefreshPeriod)*time.Second
		st.Pairs = res.Pairs
		st.Exclusions = res.Exclusions
		if !st.Fresh {
			st.Error = "last result is stale"
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		st.Error = "no valid result is available"
	} else {
		writeError(w, 500, "request failed")
		return
	}
	state, stateErr := a.store.GetRefreshState(id)
	if stateErr == nil {
		st.AttemptedAt = &state.AttemptedAt
		if state.Error != "" {
			st.Error = state.Error
		}
	} else if !errors.Is(stateErr, sql.ErrNoRows) {
		writeError(w, 500, "request failed")
		return
	}
	global, globalErr := a.store.GetServiceRefreshState()
	if globalErr == nil && global.Error != "" && (st.UpdatedAt == nil || global.AttemptedAt.After(*st.UpdatedAt)) {
		if errors.Is(stateErr, sql.ErrNoRows) || state.Error == "" || !state.AttemptedAt.After(global.AttemptedAt) {
			st.AttemptedAt = &global.AttemptedAt
			st.Error = global.Error
		}
	} else if globalErr != nil && !errors.Is(globalErr, sql.ErrNoRows) {
		writeError(w, 500, "request failed")
		return
	}
	writeJSON(w, 200, st)
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
