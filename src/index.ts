import express, { Request, Response } from "express";
import axios from "axios";
import axiosRetry from "axios-retry";
import cors from "cors";
import ccxt from "ccxt";
import dotenv from "dotenv";
import fs from "fs";
import path from "path";

dotenv.config();
const router = express.Router();

const PORT = process.env.PORT || 8080;
const BEARER_TOKEN = process.env.LUNARCRUSH_TOKEN;
const CACHE_DIR = process.env.CACHE_DIR || path.join(__dirname, "cache");
const CACHE_FILE_PATH = path.join(CACHE_DIR, "cache.json");
const CACHE_TTL_EXCHANGE =
  parseInt(process.env.CACHE_TTL_EXCHANGE || "0") || 60 * 60 * 1000; // 60 minutes
const CACHE_TTL_LUNARCRUSH =
  parseInt(process.env.CACHE_TTL_LUNARCRUSH || "0") || 1 * 60 * 1000; // 1 minute

if (!fs.existsSync(CACHE_DIR)) {
  fs.mkdirSync(CACHE_DIR, { recursive: true });
}

if (!BEARER_TOKEN) throw new Error("LunarCrush token not provided");

interface CacheData<T> {
  data: T;
  timestamp: number;
}

interface Market {
  base: string;
  quote: string;
  type: string;
  active: boolean;
}

interface LunarcrushCoin {
  symbol: string;
  [key: string]: any;
}

interface Cache {
  lunarcrush: CacheData<any> | null;
  exchanges: { [key: string]: CacheData<string[]> };
}

enum MarketType {
  SPOT = "spot",
  FUTURES = "futures",
}

const CCXTMarketType = {
  [MarketType.SPOT]: "spot",
  [MarketType.FUTURES]: "swap",
};

enum LunarcrushMode {
  ALT_RANK = "alt_rank",
  GALAXY_SCORE = "galaxy_score",
  SENTIMENT = "sentiment",
  GALAXY_SCORE_PERC = "galaxy_score_perc",
  ALT_RANK_PERC = "alt_rank_perc",
}

const SUPPORTED_EXCHANGES = {
  binance: new ccxt.binance({ enableRateLimit: true }),
  bybit: new ccxt.bybit({ enableRateLimit: true }),
  hyperliquid: new ccxt.hyperliquid({ enableRateLimit: true }),
};

axiosRetry(axios, {
  retries: 5,
  retryDelay: axiosRetry.exponentialDelay,
  retryCondition: (error) =>
    // Give up when the request is a rate limit error
    error.response?.status !== 429,
});

const loadCacheFromFile = (): Cache => {
  try {
    if (fs.existsSync(CACHE_FILE_PATH)) {
      const data = fs.readFileSync(CACHE_FILE_PATH, "utf8");
      return JSON.parse(data);
    }
  } catch (error) {
    console.error("❌ Error loading cache from file:", error);
  }
  return { lunarcrush: null, exchanges: {} };
};

const cache: Cache = loadCacheFromFile();

const saveCacheToFile = () => {
  try {
    fs.writeFileSync(CACHE_FILE_PATH, JSON.stringify(cache, null, 2), "utf8");
    console.log("💾 Cache saved to file.");
  } catch (error) {
    console.error("❌ Error saving cache to file:", error);
  }
};

export const fetchExchangePairs = async (
  exchangeName: string,
  quoteAsset = "USDT",
  marketType = MarketType.SPOT
) => {
  if (!SUPPORTED_EXCHANGES[exchangeName]) {
    throw new Error(`Exchange ${exchangeName} is not supported.`);
  }

  const cacheKey = `${exchangeName}_${marketType}_${quoteAsset}`;
  if (
    cache.exchanges[cacheKey] &&
    Date.now() - cache.exchanges[cacheKey].timestamp < CACHE_TTL_EXCHANGE
  ) {
    console.log(`⚡ Returning cached ${exchangeName} data for ${cacheKey}`);
    return cache.exchanges[cacheKey].data;
  }

  try {
    const exchange = SUPPORTED_EXCHANGES[exchangeName];
    await exchange.loadMarkets();
    const markets = Object.values(exchange.markets) as Market[];

    const pairs = markets
      .filter(
        (market) =>
          market.type === CCXTMarketType[marketType] &&
          market.quote === quoteAsset &&
          market.active
      )
      .map((market) => `${market.base}/${market.quote}`)
      .map((pair) =>
        marketType === MarketType.FUTURES ? `${pair}:${quoteAsset}` : pair
      );

    cache.exchanges[cacheKey] = { data: pairs, timestamp: Date.now() };
    return pairs;
  } catch (error) {
    console.error(`❌ Error fetching ${exchangeName} pairs:`, error);
    throw new Error(`Failed to fetch ${exchangeName} data`);
  }
};

export const fetchLunarPair = async (pair: string) => {};

export const fetchLunarcrushCoins = async () => {
  if (
    cache.lunarcrush &&
    Date.now() - cache.lunarcrush.timestamp < CACHE_TTL_LUNARCRUSH
  ) {
    console.log(`⚡ Returning cached LunarCrush data`);
    return cache.lunarcrush.data;
  }

  try {
    const response = await axios.get(
      "https://lunarcrush.com/api4/public/coins/list/v1",
      {
        headers: { Authorization: `Bearer ${BEARER_TOKEN}` },
      }
    );

    const data = response.data.data || [];
    if (!data.length)
      throw new Error("LunarCrush API returned an empty dataset.");

    cache.lunarcrush = { data, timestamp: Date.now() };
    return data;
  } catch (error) {
    console.error("❌ Error fetching LunarCrush data:", error);
    throw new Error("Failed to fetch LunarCrush data");
  }
};

router.get(
  "/pairs/:exchange/:marketType/:lunarMode",
  async (req: Request, res: Response) => {
    try {
      const exchangeName = req.params.exchange.toLowerCase();
      const limit = parseInt(req.query.limit as string, 10) || 50;
      const quoteAsset = (req.query.quoteAsset as string) || "USDT";
      const marketType =
        (req.params.marketType as MarketType) || MarketType.FUTURES;
      const lunarMode =
        (req.params.lunarMode as LunarcrushMode) || LunarcrushMode.ALT_RANK;

      console.log(`🔍 Fetching data for ${exchangeName} with params:`, {
        quoteAsset,
        marketType,
        lunarMode,
      });

      const [exchangePairs, lunarcrushData] = await Promise.all([
        fetchExchangePairs(exchangeName, quoteAsset, marketType),
        fetchLunarcrushCoins(),
      ]);

      const sortedLunarcrush = lunarcrushData.sort(
        (a: any, b: any) => a[lunarMode] - b[lunarMode]
      );

      const lunarcrushCoins: string[] = sortedLunarcrush.map(
        (coin: LunarcrushCoin) => coin.symbol
      );

      const intersection = lunarcrushCoins
        .map(
          (coin: string) =>
            exchangePairs
              .map((pair) => pair.replace(/^1(?:0)+/, ""))
              .find((pair) => pair.startsWith(`${coin}/`)) || null
        )
        .filter(Boolean);

      console.log(
        `✅ Matching Pairs:`,
        intersection.length,
        JSON.stringify(intersection.slice(0, limit))
      );

      res.json({
        pairs: intersection.slice(0, limit),
        refresh_period: Math.min(
          CACHE_TTL_LUNARCRUSH / 1000,
          CACHE_TTL_EXCHANGE / 1000
        ),
      });
    } catch (error) {
      console.error(`❌ Error in /pairs/:exchange/:marketType:`, error);
      res.status(500).json({ error: error.message });
    }
  }
);

router.get(
  "/pair/:exchange/:marketType/:pair",
  async (req: Request, res: Response) => {
    try {
      const exchangeName = req.params.exchange.toLowerCase();
      const limit = parseInt(req.query.limit as string, 10) || 50;
      const quoteAsset = (req.query.quoteAsset as string) || "USDT";
      const pair = req.query.pair as string;
      if (!pair) throw new Error("Pair not provided");
      const marketType =
        (req.params.marketType as MarketType) || MarketType.FUTURES;

      const [lunarPair] = await Promise.all([
        fetchExchangePairs(exchangeName, quoteAsset, marketType),
        fetchLunarPair(pair),
      ]);

      const intersection = lunarPair.filter((p) => p.includes(pair));

      res.json({
        pairs: intersection.slice(0, limit),
        refresh_period: Math.min(
          CACHE_TTL_LUNARCRUSH / 1000,
          CACHE_TTL_EXCHANGE / 1000
        ),
      });
    } catch (error) {
      console.error(`❌ Error in /fetchPair/:exchange/:marketType:`, error);
      res.status(500).json({ error: error.message });
    }
  }
);

const handleExit = () => {
  saveCacheToFile();
  console.log("👋 Exiting. Cache saved.");
  process.exit();
};

process.on("exit", handleExit);
process.on("SIGINT", handleExit);
process.on("SIGTERM", handleExit);

const app = express();

app.use(cors());
app.use(express.json());
app.use("/api", router);

app.listen(PORT, () => console.log(`🚀 Server running on port ${PORT}`));
export default app;