import axios from "axios";
import { cache } from "@/utils/cacheHandler";
import { ENV } from "@/config/env";
import rateLimit from "axios-rate-limit";

// API Configuration
const apiClient = axios.create({
  baseURL: "https://lunarcrush.com/api4/public",
  headers: { Authorization: `Bearer ${ENV.BEARER_TOKEN}` },
});

// Rate Limit Setup
const plan = ENV.PLAN_LUNARCRUSH;
const rateLimitPerDay =
  plan === "discover" ? 2_000 : plan === "pro" ? 20_000 : 0;
if (!rateLimitPerDay) {
  throw new Error("Invalid lunarcrush plan selected.");
}
const requestsPerMinute = rateLimitPerDay / (24 * 60);

const limitedApiClient = rateLimit(apiClient, {
  maxRequests: requestsPerMinute,
  perMilliseconds: 60 * 1000, // 1-minute window
});

export const cacheTTL = Math.max(60_000 / requestsPerMinute, 10_000); // Minimum 10s

export const fetchLunarcrushCoins = async () => {
  const cachedData = cache.lunarcrushPairs;

  if (cachedData) {
    const timeElapsed = Date.now() - cachedData.timestamp;
    const timeUntilExpiry = Math.max(
      0,
      Math.ceil((cacheTTL - timeElapsed) / 1000)
    );

    if (timeElapsed < cacheTTL) {
      console.log(
        `⚡️ Returning cached LunarCrush data. Time until expiry: ${timeUntilExpiry} seconds.`
      );
      return cachedData.data;
    }
  }

  try {
    const response = await limitedApiClient.get("/coins/list/v1");

    const data = response.data?.data || [];
    if (!data.length)
      throw new Error("LunarCrush API returned an empty dataset.");

    cache.lunarcrushPairs = { data, timestamp: Date.now() };
    console.log("🍀 Returning fresh LunarCrush data");
    return data;
  } catch (error) {
    if (error.response?.status === 429 && cachedData) {
      console.warn("⚠️ Rate limit hit! Returning cached data.");
      return cachedData.data;
    }

    console.error("❌ Error fetching LunarCrush data:", error);
    throw new Error("Failed to fetch LunarCrush data");
  }
};

export const fetchLunarcrushCoin = async (baseAsset: string) => {
  const cachedData = cache.lunarCrushHistory;
};