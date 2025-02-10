import dotenv from "dotenv";
import path from "path";

dotenv.config();

type Plans = "discover" | "pro";

interface EnvType {
  PORT: number;
  HOST: string | null;
  HTTPS: "http" | "https" | null;
  BEARER_TOKEN: string;
  CACHE_FILE_PATH: string;
  CACHE_TTL_EXCHANGE: number;
  PLAN_LUNARCRUSH: "discover" | "pro";
  API_PREFIX: string;
}

export const ENV: EnvType = {
  PORT: parseInt(process.env.PORT || "8080"),
  HOST: null,
  HTTPS: null,
  BEARER_TOKEN: process.env.LUNARCRUSH_TOKEN || "",
  CACHE_FILE_PATH: path.join(
    process.env.CACHE_DIR || path.join(__dirname, "../cache"),
    "cache.json"
  ),
  CACHE_TTL_EXCHANGE:
    parseInt(process.env.CACHE_TTL_EXCHANGE || "0") / 1000 || 3600000, // 60 min
  PLAN_LUNARCRUSH: (process.env.PLAN_LUNARCRUSH as Plans) || "discover",
  API_PREFIX: process.env.API_PREFIX || "",
};

if (!ENV.BEARER_TOKEN) throw new Error("LunarCrush token not provided");
