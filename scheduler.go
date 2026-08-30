package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type AssetCollector interface {
	Assets(context.Context) ([]Asset, error)
}
type MarketCollector interface {
	Markets(context.Context) ([]HyperMarket, error)
}

var ErrSnapshotUnavailable = errors.New("shared snapshot is not ready")

type Scheduler struct {
	store    *Store
	assets   AssetCollector
	markets  MarketCollector
	interval time.Duration
	mu       sync.RWMutex
	snapshot struct {
		assets  []Asset
		markets map[string]string
		at      time.Time
		ready   bool
	}
	trigger chan struct{}
}

func NewScheduler(s *Store, a AssetCollector, m MarketCollector, d time.Duration) *Scheduler {
	sch := &Scheduler{store: s, assets: a, markets: m, interval: d, trigger: make(chan struct{}, 1)}
	var assets []Asset
	var markets []HyperMarket
	assetsAt, assetsErr := s.LoadSnapshot("lunarcrush", &assets)
	marketsAt, marketsErr := s.LoadSnapshot("hyperliquid", &markets)
	mapped, mappingErr := BuildMarkets(markets)
	if assetsErr == nil && marketsErr == nil && mappingErr == nil && assetsAt.Equal(marketsAt) {
		sch.snapshot.assets = assets
		sch.snapshot.markets = mapped
		sch.snapshot.at = assetsAt
		sch.snapshot.ready = true
	}
	return sch
}

func staticActiveMarkets(active map[string]string) map[string]string {
	allowed := Static10Markets()
	out := make(map[string]string, len(allowed))
	for symbol, pair := range allowed {
		if active[symbol] == pair {
			out[symbol] = pair
		}
	}
	return out
}

func (s *Scheduler) evaluate(id string, config Config, now time.Time) error {
	s.mu.RLock()
	if !s.snapshot.ready {
		s.mu.RUnlock()
		return ErrSnapshotUnavailable
	}
	assets := append([]Asset(nil), s.snapshot.assets...)
	markets := s.snapshot.markets
	snapshotAt := s.snapshot.at
	s.mu.RUnlock()
	if config.Universe == "static10" {
		markets = staticActiveMarkets(markets)
	}
	pairs, excluded, err := Select(config, assets, markets)
	if err != nil {
		_ = s.store.SetRefreshState(id, "selection produced no valid pairlist", now)
		s.store.Audit(id, "selection_rejected", "empty or invalid selection")
		return err
	}
	if err := s.store.PublishResult(id, pairs, excluded, snapshotAt, now); err != nil {
		return err
	}
	s.store.Audit(id, "result_published", fmt.Sprintf("%d pairs", len(pairs)))
	return nil
}

func (s *Scheduler) EvaluateConfig(id string, config Config) error {
	return s.evaluate(id, config, time.Now().UTC())
}

func (s *Scheduler) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

func (s *Scheduler) evaluateAll(now time.Time) error {
	configs, err := s.store.ListConfigs()
	if err != nil {
		return err
	}
	var failures int
	for _, x := range configs {
		if err := s.evaluate(x.ID, x.Config, now); err != nil {
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d configurations had no valid result", failures)
	}
	return nil
}

func (s *Scheduler) handleTrigger(ctx context.Context) error {
	s.mu.RLock()
	ready := s.snapshot.ready
	s.mu.RUnlock()
	if !ready {
		return s.Refresh(ctx)
	}
	return s.evaluateAll(time.Now().UTC())
}

func (s *Scheduler) Refresh(ctx context.Context) error {
	configs, err := s.store.ListConfigs()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	markAllFailed := func() {
		_ = s.store.SetServiceRefreshState("upstream refresh failed", now)
		for _, x := range configs {
			_ = s.store.SetRefreshState(x.ID, "upstream refresh failed", now)
		}
	}
	assets, err := s.assets.Assets(ctx)
	if err != nil {
		markAllFailed()
		s.store.Audit("", "refresh_failed", "lunarcrush unavailable")
		return errors.New("asset refresh failed")
	}
	markets, err := s.markets.Markets(ctx)
	if err != nil {
		markAllFailed()
		s.store.Audit("", "refresh_failed", "hyperliquid unavailable")
		return errors.New("market refresh failed")
	}
	hm, err := BuildMarkets(markets)
	if err != nil {
		markAllFailed()
		return errors.New("market mapping failed")
	}
	if err = s.store.SaveSharedSnapshots(assets, markets, now); err != nil {
		markAllFailed()
		return err
	}
	s.mu.Lock()
	s.snapshot.assets = append([]Asset(nil), assets...)
	s.snapshot.markets = hm
	s.snapshot.at = now
	s.snapshot.ready = true
	s.mu.Unlock()
	return s.evaluateAll(now)
}
func (s *Scheduler) Run(ctx context.Context) {
	_ = s.Refresh(ctx)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.Refresh(ctx)
		case <-s.trigger:
			_ = s.handleTrigger(ctx)
		}
	}
}
