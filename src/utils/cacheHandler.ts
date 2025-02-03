import fs from "fs";
import { ENV } from "../config/env";

export interface CacheData<T> {
  data: T;
  timestamp: number;
}

export interface Cache {
  lunarcrush: CacheData<any> | null;
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
  return { lunarcrush: null, exchanges: {} };
};

export const saveCacheToFile = (cache: Cache) => {
  try {
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
