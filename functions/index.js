const functions = require("firebase-functions");
const express = require("express");
const axios = require("axios");
const cors = require("cors");
const ccxt = require("ccxt");
require("dotenv").config();

const app = express();
app.use(cors());
app.use(express.json());

const BEARER_TOKEN = process.env.LUNARCRUSH_TOKEN;
if (!BEARER_TOKEN) throw new Error("LunarCrush token not provided");

const cache = {
  lunarcrush: {},
  exchanges: {},
};
const CACHE_TTL = 1 * 60 * 1000; // 1 minute(s)

const MarketType = {
  SPOT: "spot",
  FUTURES: "futures",
};

const CCXTMarketType = {
  [MarketType.SPOT]: "spot",
  [MarketType.FUTURES]: "swap",
};

const LunarcrushMode = {
  ALT_RANK: "alt_rank",
  GALAXY_SCORE: "galaxy_score",
  SENTIMENT: "sentiment",
  GALAXY_SCORE_PERC: "galaxy_score_perc",
  ALT_RANK_PERC: "alt_rank_perc",
};

const SUPPORTED_EXCHANGES = {
  binance: new ccxt.binance(),
  bybit: new ccxt.bybit({
    enableRateLimit: true,
  }),
  hyperliquid: new ccxt.hyperliquid(),
};

const fetchExchangePairs = async (
  exchangeName,
  quoteAsset = "USDT",
  marketType = MarketType.SPOT
) => {
  if (!SUPPORTED_EXCHANGES[exchangeName]) {
    const choices = Object.keys(SUPPORTED_EXCHANGES).join(", ");
    throw new Error(`Exchange ${exchangeName} is not supported. (${choices})`);
  }

  if (!Object.values(MarketType).includes(marketType)) {
    const choices = Object.values(MarketType).join(", ");
    throw new Error(
      `Invalid market type: ${marketType} (${choices} are valid)`
    );
  }

  const cacheKey = `${exchangeName}_${marketType}_${quoteAsset}`;
  if (
    cache.exchanges[cacheKey] &&
    Date.now() - cache.exchanges[cacheKey].timestamp < CACHE_TTL
  ) {
    console.log(`⚡ Returning cached ${exchangeName} data for ${cacheKey}`);
    return cache.exchanges[cacheKey].data;
  }

  try {
    const exchange = SUPPORTED_EXCHANGES[exchangeName];
    await exchange.loadMarkets();
    const markets = Object.values(exchange.markets);

    let pairs = markets
      .filter((market) => market.type === CCXTMarketType[marketType])
      .filter((market) => market.quote === quoteAsset && market.active)
      .map((market) => `${market.base}/${market.quote}`)
      .map((pair) =>
        marketType == MarketType.FUTURES ? `${pair}:${quoteAsset}` : pair
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
    cache.lunarcrush.data &&
    Date.now() - cache.lunarcrush.timestamp < CACHE_TTL
  ) {
    console.log(`⚡ Returning cached LunarCrush data`);
    return cache.lunarcrush.data;
  }

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
};

const sortLunarcrushData = (data, mode) => {
  if (mode === LunarcrushMode.ALT_RANK) {
    return data
      .filter((d) => d.alt_rank)
      .sort((a, b) => a.alt_rank - b.alt_rank);
  } else if (mode === LunarcrushMode.GALAXY_SCORE) {
    return data
      .filter((d) => d.galaxy_score)
      .sort((a, b) => b.galaxy_score - a.galaxy_score);
  } else if (mode === LunarcrushMode.SENTIMENT) {
    return data
      .filter((d) => d.sentiment)
      .sort((a, b) => b.sentiment - a.sentiment);
  } else if (mode === LunarcrushMode.GALAXY_SCORE_PERC) {
    return data
      .filter((d) => d.galaxy_score && d.galaxy_score_previous)
      .map((d) => ({
        ...d,
        galaxy_score_change:
          (d.galaxy_score - d.galaxy_score_previous) / d.galaxy_score_previous,
      }))
      .sort((a, b) => b.galaxy_score_change - a.galaxy_score_change);
  } else if (mode === LunarcrushMode.ALT_RANK_PERC) {
    return data
      .filter((d) => d.alt_rank && d.alt_rank_previous)
      .map((d) => ({
        ...d,
        alt_rank_change:
          (d.alt_rank_previous - d.alt_rank) / d.alt_rank_previous,
      }))
      .sort((a, b) => b.alt_rank_change - a.alt_rank_change);
  } else {
    const choices = Object.values(LunarcrushMode).join(", ");
    throw new Error(`Invalid mode: ${mode} (${choices} are valid)`);
  }
};

app.get("/fetchPairs/:exchange", async (req, res) => {
  try {
    const exchangeName = req.params.exchange.toLowerCase();
    const limit = parseInt(req.query.limit) || 50;
    const quoteAsset = req.query.quoteAsset || "USDT";
    const marketType = req.query.marketType || MarketType.FUTURES;
    const lunarMode = req.query.lunarMode || LunarcrushMode.ALT_RANK;

    console.log(`🔍 Fetching data for ${exchangeName} with params:`, {
      quoteAsset,
      marketType,
      lunarMode,
    });

    const [exchangePairs, lunarcrushData] = await Promise.all([
      fetchExchangePairs(exchangeName, quoteAsset, marketType),
      fetchLunarcrushCoins(),
    ]);

    const sortedLunarcrush = sortLunarcrushData(lunarcrushData, lunarMode);
    const lunarcrushCoins = sortedLunarcrush.map((coin) => coin.symbol);

    console.log(`🔹 ${exchangeName} Pairs (${marketType}):`, exchangePairs);
    console.log(`🔹 LunarCrush Coins (${lunarMode}):`, lunarcrushCoins);

    const intersection = lunarcrushCoins
      .map(
        (coin) =>
          exchangePairs.find((pair) => pair.startsWith(`${coin}/`)) || null
      )
      .filter(Boolean);

    console.log(`✅ Matching Pairs:`, intersection);

    res.json({
      pairs: intersection.slice(0, limit),
      refresh_period: CACHE_TTL / 1000,
    });
  } catch (error) {
    console.error(`❌ Error in /fetchPairs/:exchange:`, error);
    res.status(500).json({ error: error.message });
  }
});

exports.api = functions.https.onRequest(app);

const PORT = process.env.PORT || 8080;
app.listen(PORT, () => console.log(`🚀 Server running on port ${PORT}`));
