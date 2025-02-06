import { Router } from "express";
import { fetchExchangePairs, MarketType } from "../services/exchange.service";
import { fetchLunarcrushCoins } from "../services/lunarcrush.service";

const router = Router();

interface LunarcrushCoin {
  [key: string]: number | string;
  symbol: string;
}

router.get("/pairs/:exchange/:marketType/:lunarMode", async (req, res) => {
  try {
    const { exchange, marketType: marketTypeParam, lunarMode } = req.params;
    const marketType = marketTypeParam as MarketType;
    const limit = parseInt(req.query.limit as string, 10) || 50;
    const quoteAsset =
      (req.query.quoteAsset as string) || exchange == "hyperliquid"
        ? "USDC"
        : "USDT";

    const [exchangePairs, lunarcrushData] = await Promise.all([
      fetchExchangePairs(exchange, quoteAsset, marketType),
      fetchLunarcrushCoins(),
    ]);

    const sortedLunarcrush = lunarcrushData.sort(
      (a: LunarcrushCoin, b: LunarcrushCoin) =>
        Number(a[lunarMode]) - Number(b[lunarMode])
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

    res.json({ pairs: intersection.slice(0, limit) });
  } catch (error) {
    res.status(500).json({ error: (error as Error).message });
  }
});

export default router;
