import * as request from "supertest";
import { app, server } from "../src/index";
import { fetchExchangePairs } from "../src/services/exchange.service";
import { fetchLunarcrushCoins } from "../src/services/lunarcrush.service";

jest.mock("../src/services/exchange.service");
jest.mock("../src/services/lunarcrush.service");

describe("Pairs API Routes", () => {
  afterAll((done) => {
    server.close(done); // Properly close server
  });

  it("should return pairs from Binance", async () => {
    (fetchExchangePairs as jest.Mock).mockResolvedValue([
      "BTC/USDT",
      "ETH/USDT",
    ]);
    (fetchLunarcrushCoins as jest.Mock).mockResolvedValue([
      { symbol: "BTC" },
      { symbol: "ETH" },
    ]);

    const res = await request
      .default(app)
      .get("/api/pairs/binance/spot/alt_rank?limit=2");

    expect(res.status).toBe(200);
    expect(res.body.pairs).toEqual(["BTC/USDT", "ETH/USDT"]);
  });

  it("should return an error for an unsupported exchange", async () => {
    (fetchExchangePairs as jest.Mock).mockRejectedValue(
      new Error("Exchange fakeExchange is not supported.")
    );

    const res = await request
      .default(app)
      .get("/api/pairs/fakeExchange/spot/alt_rank");

    expect(res.status).toBe(500);
    expect(res.body.error).toContain("Exchange fakeExchange is not supported.");
  });
});
