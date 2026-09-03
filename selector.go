package main

import (
	"errors"
	"sort"
	"strings"
)

type Asset struct {
	Symbol  string             `json:"symbol"`
	Metrics map[string]float64 `json:"metrics"`
}
type HyperMarket struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

var staticSymbols = []string{"BTC", "ETH", "SOL", "XRP", "DOGE", "AVAX", "LINK", "SUI", "AAVE", "LTC"}

func Static10Markets() map[string]string {
	m := map[string]string{}
	for _, s := range staticSymbols {
		m[s] = s + "/USDC:USDC"
	}
	return m
}
func BuildMarkets(in []HyperMarket) (map[string]string, error) {
	m := map[string]string{}
	counts := map[string]int{}
	for _, x := range in {
		if !x.Active || strings.ContainsAny(x.Name, "/:") || x.Name == "" {
			continue
		}
		k := strings.ToUpper(x.Name)
		counts[k]++
	}
	for k, count := range counts {
		if count != 1 {
			continue
		}
		m[k] = k + "/USDC:USDC"
	}
	return m, nil
}
func Select(c Config, assets []Asset, markets map[string]string) ([]string, map[string]string, error) {
	c, err := c.Canonical()
	if err != nil {
		return nil, nil, err
	}
	type candidate struct {
		sym, pair string
		v         float64
	}
	var cs []candidate
	ex := map[string]string{}
	counts := map[string]int{}
	for _, a := range assets {
		sym := strings.ToUpper(strings.TrimSpace(a.Symbol))
		if sym != "" {
			counts[sym]++
		}
	}
	filterKeys := make([]string, 0, len(c.Filters))
	for k := range c.Filters {
		filterKeys = append(filterKeys, k)
	}
	sort.Strings(filterKeys)
	for _, a := range assets {
		sym := strings.ToUpper(strings.TrimSpace(a.Symbol))
		if sym == "" {
			continue
		}
		if counts[sym] > 1 {
			ex[sym] = "ambiguous asset symbol"
			continue
		}
		pair, ok := markets[sym]
		if !ok {
			ex[sym] = "not in active universe"
			continue
		}
		v, ok := a.Metrics[c.SortMetric]
		if !ok {
			ex[sym] = "missing sort metric"
			continue
		}
		filtered := false
		for _, k := range filterKeys {
			r := c.Filters[k]
			n, ok := a.Metrics[k]
			if !ok || (r.Min != nil && n < *r.Min) || (r.Max != nil && n > *r.Max) {
				ex[sym] = "filtered by " + k
				filtered = true
				break
			}
		}
		if !filtered {
			cs = append(cs, candidate{sym, pair, v})
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].v == cs[j].v {
			return cs[i].sym < cs[j].sym
		}
		if c.Order == "asc" {
			return cs[i].v < cs[j].v
		}
		return cs[i].v > cs[j].v
	})
	if len(cs) > c.Limit {
		for _, x := range cs[c.Limit:] {
			ex[x.sym] = "outside result limit"
		}
		cs = cs[:c.Limit]
	}
	pairs := make([]string, len(cs))
	for i, x := range cs {
		pairs[i] = x.pair
	}
	if len(pairs) == 0 {
		return nil, ex, errors.New("selection produced no pairs")
	}
	return pairs, ex, nil
}
