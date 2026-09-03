# LunarPair

LunarPair is a small RemotePairList builder for Freqtrade. It ranks LunarCrush assets, intersects them with Hyperliquid perpetual markets, and publishes stable `SYMBOL/USDC:USDC` pairlists. The service is one Go process with an embedded web interface and SQLite persistence.

## Architecture

- `net/http` serves the API and embedded HTML, CSS, and vanilla JavaScript.
- One central scheduler fetches a shared LunarCrush snapshot and a shared Hyperliquid market snapshot. Configurations do not make independent upstream requests.
- A deterministic selector applies metric ranges, ordering, symbol matching, and limits.
- SQLite runs in WAL mode under `DATA_DIR`. It stores canonical configurations, snapshots, last-known-good results, exclusions, and audit events.
- The API publishes a result only when selection returns at least one pair. A failed fetch or empty selection does not overwrite the last-known-good result.

The opaque configuration ID is derived from canonical configuration bytes. It is stable and deduplicates identical configurations. **It is an identifier, not an authentication secret.**

## Quick start

A LunarCrush API v4 key is required for live collection.

```sh
cp .env.example .env
# Edit .env and replace the placeholders.
docker compose up --build -d
curl --fail http://localhost:8080/readyz
```

Open <http://localhost:8080>. Persistent state is in the `pairlist-data` volume. The Compose example sets a 64 MiB memory limit, drops Linux capabilities, and uses a read-only root filesystem.

### Environment variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `LUNARCRUSH_API_KEY` | yes | none | LunarCrush API v4 Bearer token. It is never stored in SQLite or returned by the API. |
| `PUBLIC_BASE_URL` | recommended | request origin | Public HTTP(S) origin used in generated URLs, for example `https://pairs.example.com`. |
| `PORT` | no | `8080` | HTTP listen port. |
| `DATA_DIR` | no | `/data` | Writable directory containing `pairlist.db`, WAL, and shared snapshots. |
| `CENTRAL_REFRESH_SECONDS` | no | `21600` | Central collection interval. Values from 300 through 86400 are accepted. |

The production clients use fixed LunarCrush and Hyperliquid API endpoints. Users cannot submit formulas or upstream URLs.

## API

### Create or retrieve a configuration

`POST /api/config` accepts one strict JSON object. Unknown fields, trailing JSON, unsupported options, nonnumeric values, and invalid ranges are rejected.

```sh
curl --fail-with-body http://localhost:8080/api/config \
  -H 'Content-Type: application/json' \
  --data '{
    "universe":"static10",
    "limit":10,
    "refresh_period":21600,
    "sort_metric":"alt_rank",
    "order":"asc",
    "filters":{
      "sentiment":{"min":60},
      "market_cap":{"min":100000000,"max":50000000000}
    }
  }'
```

Supported universes are `static10` and `hyperliquid`. `static10` is BTC, ETH, SOL, XRP, DOGE, AVAX, LINK, SUI, AAVE, and LTC, intersected with the currently active Hyperliquid perpetual markets. The dynamic universe starts with active Hyperliquid perpetual markets and intersects them with the LunarCrush snapshot coverage.

`limit` is 1–50. `refresh_period` is 21600, 43200, or 86400 seconds. Metrics are `alt_rank`, `galaxy_score`, `sentiment`, `interactions_24h`, `social_volume_24h`, `volume_24h`, and `market_cap`. Order is `asc` or `desc`. Every metric supports optional numeric `min` and `max` filters. `contributors_active` is not exposed by the list endpoint used by this low-request collector and is therefore not offered in version 1.

Response:

```json
{
  "id": "stable_opaque_id_here",
  "pairlist_url": "https://pairs.example.com/api/pairlist/stable_opaque_id_here",
  "status_url": "https://pairs.example.com/api/status/stable_opaque_id_here",
  "freqtrade_config": "{\"method\":\"RemotePairList\",\"mode\":\"whitelist\",\"processing_mode\":\"filter\",\"pairlist_url\":\"https://pairs.example.com/api/pairlist/stable_opaque_id_here\",\"number_assets\":10,\"refresh_period\":21600,\"keep_pairlist_on_failure\":true,\"read_timeout\":10,\"save_to_file\":\"user_data/last_lunarcrush_pairlist.json\"}"
}
```

Use the returned `freqtrade_config` object in Freqtrade's `pairlists` array:

```json
{
  "pairlists": [
    {
      "method": "RemotePairList",
      "mode": "whitelist",
      "processing_mode": "filter",
      "pairlist_url": "https://pairs.example.com/api/pairlist/stable_opaque_id_here",
      "number_assets": 10,
      "refresh_period": 21600,
      "keep_pairlist_on_failure": true,
      "read_timeout": 10,
      "save_to_file": "user_data/last_lunarcrush_pairlist.json"
    }
  ]
}
```

### Read a pairlist

`GET /api/pairlist/{id}` returns exactly:

```json
{"pairs":["BTC/USDC:USDC","ETH/USDC:USDC"],"refresh_period":21600}
```

It returns `503` until a valid result exists. After a valid result exists, refresh failures preserve and serve that last-known-good list.

### Inspect status

`GET /api/status/{id}` returns the sanitized configuration, creation/result timestamps, freshness and error state, selected pairs, and per-symbol exclusion reasons. It never returns the LunarCrush key.

### Probes

- `GET /healthz`: process liveness
- `GET /readyz`: SQLite readiness

Unsupported methods return `405` through Go's method-aware router. API errors contain a short public message and do not expose internal errors.

## Coolify deployment

1. Create a new Dockerfile-based resource from this repository and branch.
2. Mount a persistent volume at `/data`.
3. Set `LUNARCRUSH_API_KEY`, `PUBLIC_BASE_URL`, and `DATA_DIR=/data` as runtime variables. Keep the key secret.
4. Expose container port `8080` and use `/healthz` as the health check path.
5. Set a 64 MiB memory limit initially. Increase it if the active-market or LunarCrush response grows significantly.
6. Deploy, then verify `/readyz`, create a configuration, and inspect its status URL before connecting Freqtrade.

Do not scale this SQLite deployment to multiple replicas sharing one volume. Use one application instance.

## Security and failure model

- Request bodies, response bodies, header sizes, client retries, backoff, and all HTTP timeouts are bounded.
- Responses include CSP, frame, MIME-sniffing, referrer, and permissions headers. CORS is not enabled.
- The runtime image is distroless, non-root, and includes CA certificates. Compose removes capabilities and prevents privilege escalation.
- LunarCrush authentication is sent only as a Bearer header to the fixed API endpoint. Secrets are not logged or persisted.
- HTTP 429 and 5xx responses are retried with bounded exponential backoff. Other upstream failures are sanitized.
- Unknown or ambiguous symbols are excluded. Sorting has a symbol tie-breaker, so identical snapshots produce identical results.
- Empty newly computed lists are rejected. Persisted valid lists survive process restarts and upstream outages.
- SQLite uses WAL, foreign keys, and a busy timeout. Configuration creation uses a serialized transaction, so concurrent identical requests deduplicate safely and the 1,000-configuration cap cannot be raced.
- Audit retention is bounded to the newest 10,000 rows. New unique configurations receive HTTP 429 after the capacity limit; existing stable IDs remain usable.
- Configuration creation is limited to 20 requests per source address per minute with a bounded in-memory limiter. Configurations that are not read or recreated for 90 days are removed when capacity is needed; active Freqtrade URLs refresh their activity timestamp.

Protect the service with normal reverse-proxy controls if configuration creation should not be public. Configuration IDs do not provide authorization.

## Development and validation

Go is pinned to 1.24.5 in CI and Docker.

```sh
gofmt -w *.go
go vet ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath ./...
docker build -t lunarcrush-pairlist:local .
```

Tests use temporary SQLite databases and `httptest` upstream servers. They do not require or simulate a successful live LunarCrush call.

## Current limitations

- One process and one SQLite database are supported; there is no clustered scheduler or database migration tool.
- There is no user authentication, per-ID access control, or manual deletion API. Persistent growth is bounded to 1,000 configurations and 10,000 audit rows, and inactive configurations expire after 90 days.
- New configurations are evaluated immediately from the latest shared snapshot. If no snapshot exists yet, one coalesced central refresh is triggered; a configuration never starts an independent per-ID collector.
- Symbol matching intentionally supports exact, case-normalized ticker matches only. Contract aliases and renamed tokens are rejected rather than guessed.
- The low-request collector requests the first 1,000 assets from `coins/list/v1`; active Hyperliquid markets outside that LunarCrush snapshot are excluded. This endpoint also supports LunarCrush Discover subscriptions.
- Upstream response compatibility is covered by fixtures, but LunarCrush subscription entitlements and live schema changes must be monitored operationally.

## License

MIT. See [LICENSE](LICENSE).
