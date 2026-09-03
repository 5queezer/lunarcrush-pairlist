package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const lunarAPIURL = "https://lunarcrush.com/api4/public/coins/list/v1"
const hyperliquidAPIURL = "https://api.hyperliquid.xyz/info"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check local health endpoint")
	flag.Parse()
	if *healthcheck {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + env("PORT", "8080") + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		return
	}
	if err := run(); err != nil {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	key := os.Getenv("LUNARCRUSH_API_KEY")
	if key == "" {
		return fmt.Errorf("LUNARCRUSH_API_KEY is required")
	}
	dataDir := env("DATA_DIR", "/data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := OpenStore(filepath.Join(dataDir, "pairlist.db"))
	if err != nil {
		return err
	}
	defer store.Close()
	opts := ClientOptions{Timeout: 10 * time.Second, MaxBytes: 4 << 20, Retries: 3, Backoff: 500 * time.Millisecond}
	scheduler := NewScheduler(store, NewLunarClient(lunarAPIURL, key, opts), NewHyperClient(hyperliquidAPIURL, opts), centralInterval())
	app := NewApp(store, scheduler)
	if rawBase := os.Getenv("PUBLIC_BASE_URL"); rawBase != "" {
		base, parseErr := validatePublicBaseURL(rawBase)
		if parseErr != nil {
			return parseErr
		}
		app.SetPublicBase(base)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go scheduler.Run(ctx)
	server := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	slog.Info("service listening", "address", server.Addr)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err = <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func validatePublicBaseURL(raw string) (string, error) {
	if strings.ContainsAny(raw, "?#") {
		return "", fmt.Errorf("PUBLIC_BASE_URL must be an HTTP(S) origin")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("PUBLIC_BASE_URL must be an HTTP(S) origin")
	}
	return strings.TrimSuffix(raw, "/"), nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func centralInterval() time.Duration {
	n, err := strconv.Atoi(env("CENTRAL_REFRESH_SECONDS", "21600"))
	if err != nil || n < 300 || n > 86400 {
		return 6 * time.Hour
	}
	return time.Duration(n) * time.Second
}
