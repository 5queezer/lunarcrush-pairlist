import { Router } from "express";
import { fetchExchangePairs, MarketType } from "@/services/exchange.service";
import { fetchLunarcrushCoins, cacheTTL } from "@/services/lunarcrush.service";
import { CryptoAsset } from "@/utils/cacheHandler";

const router = Router();

interface LunarcrushCoin {
  [key: string]: number | string;
  symbol: string;
}

router.get("/lunar/:exchange/:marketType/:lunarMode", async (req, res) => {
  try {
    const { exchange, marketType: marketTypeParam, lunarMode } = req.params;
    const marketType = marketTypeParam as MarketType;
    const limit = parseInt(req.query.limit as string) || 50;
    const min = parseInt(req.query.min as string) || null;
    const max = parseInt(req.query.max as string) || null;
    const reverse =
      String(req.query.reverse || "false").toLowerCase() == "true" ||
      req.query.reverse == "1";
    const quoteAsset =
      (req.query.quoteAsset as string) || exchange == "hyperliquid"
        ? "USDC"
        : "USDT";

    const [exchangePairs, lunarcrushData] = await Promise.all([
      fetchExchangePairs(exchange, quoteAsset, marketType),
      fetchLunarcrushCoins(),
    ]);

    const sortedLunarcrush = lunarcrushData
      .slice()
      .filter((coin: CryptoAsset) => {
        const value = coin[lunarMode];
        return (max === null || value <= max) && (min === null || value >= min);
      })
      .sort((a: LunarcrushCoin, b: LunarcrushCoin) =>
        reverse
          ? Number(b[lunarMode]) - Number(a[lunarMode])
          : Number(a[lunarMode]) - Number(b[lunarMode])
      );

    const lunarcrushCoins = sortedLunarcrush.map(
      (coin: { symbol: string }) => coin.symbol
    );

    const intersection = lunarcrushCoins
      .map(
        (coin: string) =>
          exchangePairs.find((pair) => pair.startsWith(`${coin}/`)) || null
      )
      .filter(Boolean);

    res.json({
      pairs: intersection.slice(0, limit),
      refresh_period: Math.ceil(cacheTTL / 1000),
    });
  } catch (error) {
    res.status(500).json({ error: (error as Error).message });
  }
});

export default router;
