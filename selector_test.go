package main

import (
	"reflect"
	"testing"
)

func TestSelectorDeterministicFilteringAndMapping(t *testing.T) {
	assets := []Asset{{Symbol: "ETH", Metrics: map[string]float64{"alt_rank": 2}}, {Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}}, {Symbol: "DOGE", Metrics: map[string]float64{"alt_rank": 3}}, {Symbol: "UNKNOWN", Metrics: map[string]float64{"alt_rank": 0}}}
	cfg := validConfig()
	cfg.Limit = 2
	cfg.Filters = map[string]Range{"alt_rank": {Min: floatPtr(1), Max: floatPtr(3)}}
	pairs, excluded, err := Select(cfg, assets, Static10Markets())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"BTC/USDC:USDC", "ETH/USDC:USDC"}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("pairs=%v", pairs)
	}
	if excluded["UNKNOWN"] != "not in active universe" {
		t.Fatalf("excluded=%v", excluded)
	}
}
func TestHyperliquidOnlyActivePerpetualAndAmbiguityRejected(t *testing.T) {
	markets, err := BuildMarkets([]HyperMarket{{Name: "BTC", Active: true}, {Name: "ETH", Active: false}, {Name: "SPOT/USDC", Active: true}, {Name: "btc", Active: true}, {Name: "SOL", Active: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(markets, map[string]string{"SOL": "SOL/USDC:USDC"}) {
		t.Fatalf("ambiguous symbol was not excluded independently: %v", markets)
	}
	markets, err = BuildMarkets([]HyperMarket{{Name: "BTC", Active: true}, {Name: "ETH", Active: false}, {Name: "SPOT/USDC", Active: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(markets, map[string]string{"BTC": "BTC/USDC:USDC"}) {
		t.Fatalf("markets=%v", markets)
	}
}
func TestSelectorRejectsEveryAmbiguousAssetSymbol(t *testing.T) {
	assets := []Asset{
		{Symbol: "BTC", Metrics: map[string]float64{"alt_rank": 1}},
		{Symbol: "btc", Metrics: map[string]float64{"alt_rank": 2}},
		{Symbol: "ETH", Metrics: map[string]float64{"alt_rank": 3}},
	}
	cfg := validConfig()
	pairs, excluded, err := Select(cfg, assets, Static10Markets())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pairs, []string{"ETH/USDC:USDC"}) {
		t.Fatalf("pairs=%v", pairs)
	}
	if excluded["BTC"] != "ambiguous asset symbol" {
		t.Fatalf("excluded=%v", excluded)
	}
}
func floatPtr(v float64) *float64 { return &v }
