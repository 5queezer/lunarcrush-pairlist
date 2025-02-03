import { Router } from "express";
import { fetchExchangePairs } from "../services/exchange.service";
import { fetchLunarcrushCoins } from "../services/lunarcrush.service";

const router = Router();

router.get("/pairs/:exchange/:marketType/:lunarMode", async (req, res) => {
  try {
    const { exchange, marketType, lunarMode } = req.params;
    const limit = parseInt(req.query.limit as string, 10) || 50;
    const quoteAsset = (req.query.quoteAsset as string) || "USDT";

    const [exchangePairs, lunarcrushData] = await Promise.all([
      fetchExchangePairs(exchange, quoteAsset, marketType),
      fetchLunarcrushCoins(),
    ]);

    const sortedLunarcrush = lunarcrushData.sort(
      (a, b) => a[lunarMode] - b[lunarMode]
    );
    const lunarcrushCoins = sortedLunarcrush.map((coin) => coin.symbol);

    const intersection = lunarcrushCoins
      .map(
        (coin) =>
          exchangePairs.find((pair) => pair.startsWith(`${coin}/`)) || null
      )
      .filter(Boolean);

    res.json({ pairs: intersection.slice(0, limit) });
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

export default router;
