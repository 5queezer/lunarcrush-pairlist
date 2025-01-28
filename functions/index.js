/**
 * Import function triggers from their respective submodules:
 *
 * const {onCall} = require("firebase-functions/v2/https");
 * const {onDocumentWritten} = require("firebase-functions/v2/firestore");
 *
 * See a full list of supported triggers at https://firebase.google.com/docs/functions
 */

const { onRequest } = require("firebase-functions/v2/https");
const logger = require("firebase-functions/logger");

// Create and deploy your first functions
// https://firebase.google.com/docs/functions/get-started

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

// Fetch Binance Coins
const fetchBinanceCoins = async (
  quoteAsset = "USDT",
  marketType = MarketType.SPOT
) => {
  if (!BinanceEndpoints[marketType]) {
    throw new Error(`Invalid market_type: ${marketType}`);
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
          symbol.quoteAsset === quoteAsset &&
          symbol.contractStatus === "TRADING"
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

  return pairs;
};

// Fetch LunarCrush Coins
const fetchLunarcrushCoins = async (mode = LunarcrushMode.ALT_RANK) => {
  const response = await axios.get(
    "https://lunarcrush.com/api4/public/coins/list/v1",
    {
      headers: { Authorization: `Bearer ${BEARER_TOKEN}` },
    }
  );

  const data = response.data.data || [];
  if (!data.length)
    throw new Error("LunarCrush API returned an empty dataset.");

  let sortedData = [];
  if (mode === LunarcrushMode.ALT_RANK) {
    sortedData = data
      .filter((d) => d.alt_rank)
      .sort((a, b) => a.alt_rank - b.alt_rank);
  } else if (mode === LunarcrushMode.GALAXY_SCORE) {
    sortedData = data
      .filter((d) => d.galaxy_score)
      .sort((a, b) => b.galaxy_score - a.galaxy_score);
  } else if (mode === LunarcrushMode.SENTIMENT) {
    sortedData = data
      .filter((d) => d.sentiment)
      .sort((a, b) => b.sentiment - a.sentiment);
  } else if (mode === LunarcrushMode.GALAXY_SCORE_PERC) {
    sortedData = data
      .filter((d) => d.galaxy_score && d.galaxy_score_previous)
      .map((d) => ({
        ...d,
        galaxy_score_change:
          (d.galaxy_score - d.galaxy_score_previous) / d.galaxy_score_previous,
      }))
      .sort((a, b) => b.galaxy_score_change - a.galaxy_score_change);
  } else if (mode === LunarcrushMode.ALT_RANK_PERC) {
    sortedData = data
      .filter((d) => d.alt_rank && d.alt_rank_previous)
      .map((d) => ({
        ...d,
        alt_rank_change:
          (d.alt_rank_previous - d.alt_rank) / d.alt_rank_previous,
      }))
      .sort((a, b) => b.alt_rank_change - a.alt_rank_change); // Higher improvement first
  } else {
    throw new Error(`Invalid mode: ${mode}`);
  }

  return sortedData.map((coin) => coin.symbol);
};

// Main API Route
app.get("/fetchPairs", async (req, res) => {
  try {
    const limit = parseInt(req.query.limit) || 50;
    const quoteAsset = req.query.quoteAsset || "USDT";
    const marketType = req.query.marketType || MarketType.FUTURES;
    const lunarMode = req.query.lunarMode || LunarcrushMode.ALT_RANK;

    const [binancePairs, lunarcrushCoins] = await Promise.all([
      fetchBinanceCoins(quoteAsset, marketType),
      fetchLunarcrushCoins(lunarMode),
    ]);

    const intersection = binancePairs.filter((pair) =>
      lunarcrushCoins.includes(pair.split("/")[0])
    );

    res.json({
      pairs: intersection.slice(0, limit),
      refresh_period: 0,
    });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

// Export Firebase Function
exports.api = functions.https.onRequest(app);
