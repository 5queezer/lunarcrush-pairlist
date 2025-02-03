import ccxt from "ccxt";
import { cache, CacheData } from "../utils/cacheHandler";
import { ENV } from "../config/env";

export enum MarketType {
  SPOT = "spot",
  FUTURES = "futures",
}

const CCXTMarketType = {
  [MarketType.SPOT]: "spot",
  [MarketType.FUTURES]: "swap",
};

const SUPPORTED_EXCHANGES = {
  binance: new ccxt.binance({ enableRateLimit: true }),
  bybit: new ccxt.bybit({ enableRateLimit: true }),
  hyperliquid: new ccxt.hyperliquid({ enableRateLimit: true }),
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
    Date.now() - cache.exchanges[cacheKey].timestamp < ENV.CACHE_TTL_EXCHANGE
  ) {
    console.log(`⚡ Returning cached ${exchangeName} data for ${cacheKey}`);
    return cache.exchanges[cacheKey].data;
  }

  try {
    const exchange = SUPPORTED_EXCHANGES[exchangeName];
    await exchange.loadMarkets();
    const markets = Object.values(exchange.markets);

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
