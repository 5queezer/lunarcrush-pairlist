package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
)

type Range struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}
type Config struct {
	Universe      string           `json:"universe"`
	Limit         int              `json:"limit"`
	RefreshPeriod int              `json:"refresh_period"`
	SortMetric    string           `json:"sort_metric"`
	Order         string           `json:"order"`
	Filters       map[string]Range `json:"filters,omitempty"`
}

var metrics = map[string]bool{"alt_rank": true, "galaxy_score": true, "sentiment": true, "interactions_24h": true, "social_volume_24h": true, "volume_24h": true, "market_cap": true}

func (c Config) Canonical() (Config, error) {
	if c.Universe != "static10" && c.Universe != "hyperliquid" {
		return c, errors.New("invalid universe")
	}
	if c.Limit < 1 || c.Limit > 50 {
		return c, errors.New("limit must be between 1 and 50")
	}
	if c.RefreshPeriod != 21600 && c.RefreshPeriod != 43200 && c.RefreshPeriod != 86400 {
		return c, errors.New("invalid refresh period")
	}
	if !metrics[c.SortMetric] {
		return c, errors.New("invalid sort metric")
	}
	if c.Order != "asc" && c.Order != "desc" {
		return c, errors.New("invalid order")
	}
	if c.Filters == nil {
		c.Filters = map[string]Range{}
	}
	for k, r := range c.Filters {
		if !metrics[k] {
			return c, errors.New("invalid filter metric")
		}
		if r.Min == nil && r.Max == nil {
			return c, errors.New("empty filter")
		}
		if r.Min != nil && (math.IsNaN(*r.Min) || math.IsInf(*r.Min, 0)) {
			return c, errors.New("invalid minimum")
		}
		if r.Max != nil && (math.IsNaN(*r.Max) || math.IsInf(*r.Max, 0)) {
			return c, errors.New("invalid maximum")
		}
		if r.Min != nil && r.Max != nil && *r.Min > *r.Max {
			return c, errors.New("minimum exceeds maximum")
		}
	}
	return c, nil
}
func (c Config) canonicalJSON() []byte {
	keys := make([]string, 0, len(c.Filters))
	for k := range c.Filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type item struct {
		Name string   `json:"name"`
		Min  *float64 `json:"min,omitempty"`
		Max  *float64 `json:"max,omitempty"`
	}
	fs := make([]item, 0, len(keys))
	for _, k := range keys {
		r := c.Filters[k]
		fs = append(fs, item{k, r.Min, r.Max})
	}
	b, _ := json.Marshal(struct {
		Universe string `json:"universe"`
		Limit    int    `json:"limit"`
		Refresh  int    `json:"refresh_period"`
		Metric   string `json:"sort_metric"`
		Order    string `json:"order"`
		Filters  []item `json:"filters"`
	}{c.Universe, c.Limit, c.RefreshPeriod, c.SortMetric, c.Order, fs})
	return b
}
func (c Config) ID() string {
	sum := sha256.Sum256(c.canonicalJSON())
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
func DecodeConfigJSON(d *json.Decoder) (Config, error) {
	var raw json.RawMessage
	if err := d.Decode(&raw); err != nil {
		return Config{}, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return Config{}, errors.New("request must contain one JSON object")
	}
	if err := rejectDuplicateKeys(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return Config{}, err
	}
	var c Config
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	if err := strict.Decode(&c); err != nil {
		return c, err
	}
	return c.Canonical()
}

func rejectDuplicateKeys(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := []string{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid JSON object key")
			}
			for _, prior := range seen {
				if strings.EqualFold(prior, key) {
					return errors.New("duplicate JSON object key")
				}
			}
			seen = append(seen, key)
			if err := rejectDuplicateKeys(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case '[':
		for d.More() {
			if err := rejectDuplicateKeys(d); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	default:
		return errors.New("invalid JSON delimiter")
	}
}
