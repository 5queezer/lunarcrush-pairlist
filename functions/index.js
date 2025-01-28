const functions = require("firebase-functions");
const express = require("express");
const axios = require("axios");
const cors = require("cors");
require("dotenv").config();

const app = express();
app.use(cors());
app.use(express.json());

const BEARER_TOKEN = process.env.LUNARCRUSH_TOKEN;
if (!BEARER_TOKEN) throw new Error("LunarCrush token not provided");

const cache = {
  lunarcrush: {},
  binance: {},
};
const CACHE_TTL = 1 * 60 * 1000; // 1 minute(s)

const MarketType = {
  SPOT: "spot",
  MARGIN: "margin",
  FUTURES: "futures",
};

const BinanceEndpoints = {
  [MarketType.SPOT]: "https://api.binance.com/api/v3/exchangeInfo",
  [MarketType.MARGIN]: "https://api.binance.com/sapi/v1/margin/allPairs",
  [MarketType.FUTURES]: "https://fapi.binance.com/fapi/v1/exchangeInfo",
};

const LunarcrushMode = {
  ALT_RANK: "alt_rank",
  GALAXY_SCORE: "galaxy_score",
  SENTIMENT: "sentiment",
  GALAXY_SCORE_PERC: "galaxy_score_perc",
  ALT_RANK_PERC: "alt_rank_perc",
};

const fetchBinanceCoins = async (
  quoteAsset = "USDT",
  marketType = MarketType.SPOT
) => {
  const cacheKey = `${marketType}_${quoteAsset}`;
  if (
    cache.binance[cacheKey] &&
    Date.now() - cache.binance[cacheKey].timestamp < CACHE_TTL
  ) {
    console.log(`⚡ Returning cached Binance data for ${cacheKey}`);
    return cache.binance[cacheKey].data;
  }

  const response = await axios.get(BinanceEndpoints[marketType]);
  const data = response.data;
  let pairs = [];

  if (marketType === MarketType.SPOT) {
    pairs = data.symbols
      .filter(
        (symbol) =>
          symbol.quoteAsset === quoteAsset && symbol.status === "TRADING"
      )
      .map((symbol) => `${symbol.baseAsset}/${symbol.quoteAsset}`);
  } else if (marketType === MarketType.FUTURES) {
    pairs = data.symbols
      .filter(
        (symbol) =>
          symbol.quoteAsset === quoteAsset && symbol.status === "TRADING"
      )
      .map(
        (symbol) =>
          `${symbol.baseAsset}/${symbol.quoteAsset}:${symbol.quoteAsset}`
      );
  } else if (marketType === MarketType.MARGIN) {
    pairs = data
      .filter((pair) => pair.quoteAsset === quoteAsset && pair.isMarginTrade)
      .map((pair) => `${pair.baseAsset}/${pair.quoteAsset}`);
  }

  cache.binance[cacheKey] = { data: pairs, timestamp: Date.now() };
  return pairs;
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
    throw new Error(`Invalid mode: ${mode}`);
  }
};

app.get("/fetchPairs", async (req, res) => {
  try {
    const limit = parseInt(req.query.limit) || 50;
    const quoteAsset = req.query.quoteAsset || "USDT";
    const marketType = req.query.marketType || MarketType.FUTURES;
    const lunarMode = req.query.lunarMode || LunarcrushMode.ALT_RANK;

    console.log(`🔍 Fetching data with params:`, {
      quoteAsset,
      marketType,
      lunarMode,
    });

    const [binancePairs, lunarcrushData] = await Promise.all([
      fetchBinanceCoins(quoteAsset, marketType),
      fetchLunarcrushCoins(),
    ]);

    const sortedLunarcrush = sortLunarcrushData(lunarcrushData, lunarMode);
    const lunarcrushCoins = sortedLunarcrush.map((coin) => coin.symbol);

    console.log(`🔹 Binance Pairs (${marketType}):`, binancePairs);
    console.log(`🔹 LunarCrush Coins (${lunarMode}):`, lunarcrushCoins);

    const intersection = lunarcrushCoins
      .map(
        (coin) =>
          binancePairs.find((pair) => pair.startsWith(`${coin}/`)) || null
      )
      .filter(Boolean);

    console.log(`✅ Matching Pairs:`, intersection);

    res.json({
      pairs: intersection.slice(0, limit),
      refresh_period: CACHE_TTL / 1000,
    });
  } catch (error) {
    console.error(`❌ Error in /fetchPairs:`, error);
    res.status(500).json({ error: error.message });
  }
});

exports.api = functions.https.onRequest(app);