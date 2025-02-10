# LunarCrush Pairlist API for Freqtrade

## For Freqtrade

## Installation

### Prerequisites
- **Bun** (>=1.x recommended)
- **Docker** (optional)
- **LunarCrush Subscription Plan Required**

### Setup
1. Clone the repository:
   ```sh
   git clone <repo-url>
   cd <repo-directory>
   ```
2. Install dependencies:
   ```sh
   bun install
   ```
3. Create a `.env` file and configure it:
   ```sh
   PORT=8080
   LUNARCRUSH_TOKEN=<your-lunarcrush-token>
   PLAN_LUNARCRUSH=discover   # Options: discover, pro
   API_PREFIX="/api"
   CACHE_DIR="./cache"
   CACHE_TTL_EXCHANGE=3600000
   ```

## Running the API
### Locally
```sh
bun run start
```

### With Docker
```sh
docker build -t crypto-api .
docker run -p 8080:8080 --env-file .env crypto-api
```

## API Endpoints

### Health Check
```
GET /api/health
Response: { "status": "OK" }
```

### Get LunarCrush Trading Pairs
```
GET /api/pairlist/lunar/:exchange/:marketType/:lunarMode

Query Params:
- limit (optional, default: 50)
- min (optional)
- max (optional)
- sort (asc/desc, default: asc)
- quoteAsset (optional, default: USDT)
```

Example:
Alt Rank: smaller is better, thus asc
```sh
GET /api/pairlist/lunar/binance/futures/alt_rank?limit=100&sort=asc
```

Galaxy score: bigger is better, thus desc
```sh
GET /api/pairlist/lunar/binance/futures/galaxy_score?limit=100&sort=desc
```


## LunarCrush API Response Example
```json
{
  "id": 169,
  "symbol": "FET",
  "name": "Fetch",
  "price": 0.7107922567775068,
  "price_btc": 0.000007451327247342504,
  "volume_24h": 124655937.65,
  "volatility": 0.0267,
  "circulating_supply": 2390224782.5,
  "max_supply": 2719493897,
  "percent_change_1h": -1.735105198471,
  "percent_change_24h": -7.26300805892,
  "percent_change_7d": -28.874032450156,
  "market_cap": 1698953267.36,
  "market_cap_rank": 71,
  "interactions_24h": 764785,
  "social_volume_24h": 6246,
  "social_dominance": 1.0931830950418213,
  "market_dominance": 0.054706720805704614,
  "market_dominance_prev": 0.053308437993835875,
  "galaxy_score": 45,
  "galaxy_score_previous": 43,
  "alt_rank": 850,
  "alt_rank_previous": 41,
  "sentiment": 79,
  "categories": "nft,ai,ai-agents",
  "blockchains": [
    [Object ...], [Object ...], [Object ...]
  ],
  "percent_change_30d": -46.45966230155,
  "last_updated_price": 1739031694,
  "last_updated_price_by": "cmc_stream",
  "topic": "fet fetch",
  "logo": "https://cdn.lunarcrush.com/fetch.png"
}
```

Regarding to this response, marketType can be one of the following:
- volume_24h
- volatility
- percent_change_1h, percent_change_24h, percent_change_7d
- market_cap, market_cap_rank
- interactions_24h, social_volume_24h
- galaxy_score, galaxy_score_previous
- alt_rank, alt_rank_previous
- sentiment
- percent_change_30d

## Environment Variables
| Variable           | Description                                | Default |
|-------------------|--------------------------------|---------|
| `PORT`           | Server port                  | `8080`  |
| `LUNARCRUSH_TOKEN` | API token for LunarCrush      | Required |
| `PLAN_LUNARCRUSH` | Subscription plan (discover/pro) | `discover` |
| `API_PREFIX`     | API URL prefix                 | `""` |
| `CACHE_DIR`     | Directory for cache storage    | `./cache` |
| `CACHE_TTL_EXCHANGE` | Exchange data cache duration (ms) | `3600000` |

## Error Handling
- If the **LunarCrush API limit** is reached, cached data is returned.
- API errors return JSON responses with appropriate HTTP status codes.
- Missing required environment variables will cause the app to throw errors.

## Deployment
- Can be deployed using **Docker**
- Compatible with **Kubernetes** and **AWS Lambda**
- Works with **Reverse Proxy (Nginx, Traefik, etc.)**

## License
MIT License
