import fs from "fs";
import path from "path";
import { ENV } from "@/config/env";

interface Blockchain {
  network: string;
  address: string;
  decimals: number;
}

export interface CryptoAsset {
  id: number;
  symbol: string;
  name: string;
  price: number;
  price_btc: number;
  volume_24h: number;
  volatility: number;
  circulating_supply: number;
  max_supply: number | null;
  percent_change_1h: number;
  percent_change_24h: number;
  percent_change_7d: number;
  market_cap: number;
  market_cap_rank: number;
  interactions_24h: number;
  social_volume_24h: number;
  social_dominance: number;
  market_dominance: number;
  market_dominance_prev: number;
  galaxy_score: number;
  galaxy_score_previous: number;
  alt_rank: number;
  alt_rank_previous: number;
  sentiment: number;
  blockchains: Blockchain[];
  topic: string;
  logo: string;
}

export interface CacheData<T> {
  data: T;
  timestamp: number;
}

export interface Cache {
  lunarcrushPairs: CacheData<CryptoAsset>;
  lunarCrushHistory: { [key: string]: CacheData<string[]> };
  exchanges: { [key: string]: CacheData<string[]> };
}

export const loadCacheFromFile = (): Cache => {
  try {
    if (fs.existsSync(ENV.CACHE_FILE_PATH)) {
      const data = fs.readFileSync(ENV.CACHE_FILE_PATH, "utf8");
      return JSON.parse(data);
    }
  } catch (error) {
    console.error("❌ Error loading cache from file:", error);
  }
  return { lunarcrushPairs: null, lunarCrushHistory: {}, exchanges: {} };
};

export const saveCacheToFile = (cache: Cache) => {
  try {
    const cacheDir = path.dirname(ENV.CACHE_FILE_PATH);

    // Ensure cache directory exists
    if (!fs.existsSync(cacheDir)) {
      fs.mkdirSync(cacheDir, { recursive: true });
    }

    fs.writeFileSync(
      ENV.CACHE_FILE_PATH,
      JSON.stringify(cache, null, 2),
      "utf8"
    );
    console.log("💾 Cache saved to file.");
  } catch (error) {
    console.error("❌ Error saving cache to file:", error);
  }
};

export const cache: Cache = loadCacheFromFile();
