import * as ccxt from "ccxt";
import { cache, CacheData } from "../utils/cacheHandler";
import { Market } from "ccxt";
import { ENV } from "../config/env";

export enum MarketType {
  SPOT = "spot",
  FUTURES = "futures",
}

export const CCXTMarketType = {
  [MarketType.SPOT]: "spot",
  [MarketType.FUTURES]: "swap",
};

export const SUPPORTED_EXCHANGES: { [key: string]: ccxt.Exchange } = {
  binance: new ccxt.binance({ enableRateLimit: true }),
  bybit: new ccxt.bybit({ enableRateLimit: true }),
  hyperliquid: new ccxt.hyperliquid({ enableRateLimit: true }),
};

export const fetchExchangePairs = async (
  exchangeName: keyof typeof SUPPORTED_EXCHANGES,
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
    const markets = Object.values(exchange.markets || {});

    const pairs = markets
      .filter(
        (market) =>
          market &&
          market.type === CCXTMarketType[marketType] &&
          market.quote === quoteAsset &&
          market.active
      )
      .map((market) => market && `${market.base}/${market.quote}`)
      .map((pair) =>
        marketType === MarketType.FUTURES ? `${pair}:${quoteAsset}` : pair
      )
      .filter((pair): pair is string => !!pair);

    cache.exchanges[cacheKey] = { data: pairs, timestamp: Date.now() };
    return pairs;
  } catch (error) {
    console.error(`❌ Error fetching ${exchangeName} pairs:`, error);
    throw new Error(`Failed to fetch ${exchangeName} data`);
  }
};
