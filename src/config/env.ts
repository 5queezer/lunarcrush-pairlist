import dotenv from "dotenv";
import path from "path";

dotenv.config();

export const ENV = {
  PORT: process.env.PORT || "8080",
  BEARER_TOKEN: process.env.LUNARCRUSH_TOKEN || "",
  CACHE_DIR: process.env.CACHE_DIR || path.join(__dirname, "../../cache"),
  CACHE_FILE_PATH: path.join(
    process.env.CACHE_DIR || path.join(__dirname, "../../cache"),
    "cache.json"
  ),
  CACHE_TTL_EXCHANGE:
    parseInt(process.env.CACHE_TTL_EXCHANGE || "0") / 1000 || 3600000, // 60 min
  CACHE_TTL_LUNARCRUSH:
    parseInt(process.env.CACHE_TTL_LUNARCRUSH || "0") / 1000 || 60000, // 1 min
};

if (!ENV.BEARER_TOKEN) throw new Error("LunarCrush token not provided");
