import axios from "axios";
import { cache } from "../utils/cacheHandler";
import { ENV } from "../config/env";

export const fetchLunarcrushCoins = async () => {
  if (
    cache.lunarcrush &&
    Date.now() - cache.lunarcrush.timestamp < ENV.CACHE_TTL_LUNARCRUSH
  ) {
    console.log(`⚡ Returning cached LunarCrush data`);
    return cache.lunarcrush.data;
  }

  try {
    const response = await axios.get(
      "https://lunarcrush.com/api4/public/coins/list/v1",
      {
        headers: { Authorization: `Bearer ${ENV.BEARER_TOKEN}` },
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
