import express, { Request, Response } from "express";
import axios from "axios";
import axiosRetry from "axios-retry";
import cors from "cors";
import ccxt, { Exchange } from "ccxt";
import dotenv from "dotenv";

dotenv.config();

const app = express();
app.use(cors());
app.use(express.json());

axiosRetry(axios, {
  retries: 5,
  retryDelay: axiosRetry.exponentialDelay,
});

const BEARER_TOKEN = process.env.LUNARCRUSH_TOKEN;
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

const cache: Cache = {
  lunarcrush: null,
  exchanges: {},
};

const CACHE_TTL_EXCHANGE = 60 * 60 * 1000; // 60 minutes
const CACHE_TTL_LUNARCRUSH = 1 * 60 * 1000; // 1 minute

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

const fetchExchangePairs = async (
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

const fetchLunarcrushCoins = async () => {
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

app.get("/fetchPairs/:exchange", async (req: Request, res: Response) => {
  try {
    const exchangeName = req.params.exchange.toLowerCase();
    const limit = parseInt(req.query.limit as string, 10) || 50;
    const quoteAsset = (req.query.quoteAsset as string) || "USDT";
    const marketType =
      (req.query.marketType as MarketType) || MarketType.FUTURES;
    const lunarMode =
      (req.query.lunarMode as LunarcrushMode) || LunarcrushMode.ALT_RANK;

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
          exchangePairs.find((pair) => pair.startsWith(`${coin}/`)) || null
      )
      .filter(Boolean);

    console.log(
      `✅ Matching Pairs:`,
      intersection.length,
      JSON.stringify(intersection.slice(0, limit))
    );

    res.json({
      pairs: intersection.slice(0, limit),
      refresh_period: CACHE_TTL_LUNARCRUSH / 1000,
    });
  } catch (error) {
    console.error(`❌ Error in /fetchPairs/:exchange:`, error);
    res.status(500).json({ error: error.message });
  }
});

const PORT = process.env.PORT || 8080;
app.listen(PORT, () => console.log(`🚀 Server running on port ${PORT}`));
