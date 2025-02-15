import axios, { AxiosError, AxiosResponse } from "axios";
import { cache } from "@/utils/cacheHandler";
import { ENV } from "@/config/env";
import rateLimit from "axios-rate-limit";

interface RateLimitInfo {
  perDay: number;
  perMinute: number;
  dayReset: number;
  minuteReset: number;
  timestamp: number;
}

interface CachedResponse {
  data: any[];
  timestamp: number;
}

const apiClient = axios.create({
  baseURL: "https://lunarcrush.com/api4/public",
  headers: { Authorization: `Bearer ${ENV.LUNARCRUSH_TOKEN}` },
});

let rateLimitInfo: RateLimitInfo | null = null;

const updateRateLimits = (response: AxiosResponse) => {
  const headers = response.headers;
  const dayLimit = parseInt(headers["x-rate-limit-day"]);
  const minuteLimit = parseInt(headers["x-rate-limit-minute"]);
  const dayReset = parseInt(headers["x-rate-limit-day-reset"]);
  const minuteReset = parseInt(headers["x-rate-limit-minute-reset"]);

  if (dayLimit && minuteLimit && dayReset && minuteReset) {
    rateLimitInfo = {
      perDay: dayLimit,
      perMinute: minuteLimit,
      dayReset,
      minuteReset,
      timestamp: Date.now(),
    };
    configureLimiter();
    if (dayLimit === 20_000 && minuteLimit === 100) {
      console.info(`💹 Plan 'Pro' detected`);
    } else {
      console.info(`🔰 Plan 'Discover' detected`);
    }
  }
};

let limitedApiClient = apiClient;
const configureLimiter = () => {
  if (!rateLimitInfo) {
    // If no rate limits are set yet, allow requests but they'll be governed by the API's limits
    limitedApiClient = apiClient;
    return;
  }

  limitedApiClient = rateLimit(apiClient, {
    maxRequests: rateLimitInfo.perMinute,
    perMilliseconds: 60 * 1000, // 1-minute window
  });
};

configureLimiter();

// Dynamic cache TTL calculation
export const getCacheTTL = () => {
  if (!rateLimitInfo) return 10_000;
  return Math.max(60_000 / rateLimitInfo.perMinute, 10_000); // 10s
};

export const fetchLunarcrushCoins = async () => {
  const cachedData = cache.lunarcrushPairs as CachedResponse;

  if (cachedData) {
    const timeElapsed = Date.now() - cachedData.timestamp;
    const timeUntilExpiry = Math.max(
      0,
      Math.ceil((getCacheTTL() - timeElapsed) / 1000)
    );

    if (timeElapsed < getCacheTTL()) {
      console.log(
        `⚡️ Returning cached LunarCrush data. Time until expiry: ${timeUntilExpiry} seconds.`
      );
      return cachedData.data;
    }
  }

  try {
    const response = await limitedApiClient.get("/coins/list/v1");

    updateRateLimits(response);

    const data = response.data?.data || [];
    if (!data.length)
      throw new Error("LunarCrush API returned an empty dataset.");

    cache.lunarcrushPairs = { data, timestamp: Date.now() };
    console.log("🍀 Returning fresh LunarCrush data");
    return data;
  } catch (error) {
    if (error instanceof AxiosError) {
      if (error.response?.status === 429 && cache.lunarcrushPairs) {
        console.warn("⚠️ Rate limit hit! Returning cached data.");
        return cache.lunarcrushPairs.data;
      }

      // Update rate limits even on error responses
      if (error.response) {
        updateRateLimits(error.response);
      }
    }
    console.error("❌ Error fetching LunarCrush data:", error);
    throw error;
  }
};
