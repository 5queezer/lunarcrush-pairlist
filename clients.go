package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ClientOptions struct {
	Timeout  time.Duration
	MaxBytes int64
	Retries  int
	Backoff  time.Duration
}

func (o ClientOptions) normalized() ClientOptions {
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Second
	}
	if o.MaxBytes <= 0 {
		o.MaxBytes = 2 << 20
	}
	if o.Retries < 0 {
		o.Retries = 0
	}
	if o.Retries > 4 {
		o.Retries = 4
	}
	if o.Backoff <= 0 {
		o.Backoff = 200 * time.Millisecond
	}
	return o
}
func doJSON(ctx context.Context, h *http.Client, reqfn func() (*http.Request, error), o ClientOptions, out any) error {
	o = o.normalized()
	var last error
	for i := 0; i <= o.Retries; i++ {
		req, err := reqfn()
		if err != nil {
			return err
		}
		req = req.WithContext(ctx)
		resp, err := h.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, o.MaxBytes+1))
			resp.Body.Close()
			if readErr != nil {
				return errors.New("upstream response read failed")
			}
			if int64(len(body)) > o.MaxBytes {
				return errors.New("upstream response too large")
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err = json.Unmarshal(body, out); err != nil {
					return errors.New("invalid upstream response")
				}
				return nil
			}
			if resp.StatusCode != 429 && resp.StatusCode < 500 {
				return fmt.Errorf("upstream returned status %d", resp.StatusCode)
			}
			last = fmt.Errorf("upstream temporarily unavailable (%d)", resp.StatusCode)
		} else {
			last = errors.New("upstream request failed")
		}
		if i < o.Retries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(o.Backoff * time.Duration(1<<i)):
			}
		}
	}
	return last
}

type LunarClient struct {
	base, key string
	o         ClientOptions
	http      *http.Client
}

func NewLunarClient(base, key string, o ClientOptions) *LunarClient {
	o = o.normalized()
	return &LunarClient{strings.TrimRight(base, "/"), key, o, &http.Client{Timeout: o.Timeout}}
}
func (c *LunarClient) Assets(ctx context.Context) ([]Asset, error) {
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	err := doJSON(ctx, c.http, func() (*http.Request, error) {
		u, err := url.Parse(c.base)
		if err != nil {
			return nil, err
		}
		query := u.Query()
		query.Set("limit", "1000")
		u.RawQuery = query.Encode()
		r, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err == nil {
			r.Header.Set("Accept", "application/json")
			r.Header.Set("Authorization", "Bearer "+c.key)
		}
		return r, err
	}, c.o, &raw)
	if err != nil {
		return nil, err
	}
	if len(raw.Data) == 0 {
		return nil, errors.New("empty LunarCrush asset data")
	}
	out := make([]Asset, 0, len(raw.Data))
	for _, x := range raw.Data {
		sym, _ := x["symbol"].(string)
		sym = strings.TrimSpace(sym)
		if sym == "" {
			return nil, errors.New("invalid LunarCrush asset record")
		}
		m := map[string]float64{}
		for k := range metrics {
			if v, ok := x[k].(float64); ok {
				m[k] = v
			}
		}
		out = append(out, Asset{sym, m})
	}
	return out, nil
}

type HyperClient struct {
	url  string
	o    ClientOptions
	http *http.Client
}

func NewHyperClient(url string, o ClientOptions) *HyperClient {
	o = o.normalized()
	return &HyperClient{url, o, &http.Client{Timeout: o.Timeout}}
}
func (c *HyperClient) Markets(ctx context.Context) ([]HyperMarket, error) {
	var raw []json.RawMessage
	err := doJSON(ctx, c.http, func() (*http.Request, error) {
		r, err := http.NewRequest(http.MethodPost, c.url, bytes.NewBufferString(`{"type":"metaAndAssetCtxs"}`))
		if err == nil {
			r.Header.Set("Accept", "application/json")
			r.Header.Set("Content-Type", "application/json")
		}
		return r, err
	}, c.o, &raw)
	if err != nil {
		return nil, err
	}
	if len(raw) != 2 {
		return nil, errors.New("invalid Hyperliquid metadata tuple")
	}
	var metadata struct {
		Universe []struct {
			Name       string `json:"name"`
			IsDelisted bool   `json:"isDelisted"`
		} `json:"universe"`
	}
	if err := json.Unmarshal(raw[0], &metadata); err != nil {
		return nil, errors.New("invalid Hyperliquid metadata")
	}
	var contexts []json.RawMessage
	if err := json.Unmarshal(raw[1], &contexts); err != nil || len(metadata.Universe) == 0 || len(contexts) != len(metadata.Universe) {
		return nil, errors.New("invalid Hyperliquid asset contexts")
	}
	out := make([]HyperMarket, 0, len(metadata.Universe))
	for _, x := range metadata.Universe {
		name := strings.TrimSpace(x.Name)
		if name == "" {
			return nil, errors.New("invalid Hyperliquid market record")
		}
		out = append(out, HyperMarket{name, !x.IsDelisted})
	}
	return out, nil
}
