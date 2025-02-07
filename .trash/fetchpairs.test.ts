import request from "supertest";
import app from "./index";
import axios from "axios";
import MockAdapter from "axios-mock-adapter";
import { fetchExchangePairs } from "./index";

const mock = new MockAdapter(axios);

const MOCK_EXCHANGE_PAIRS = ["BTC/USDT", "ETH/USDT", "DOGE/USDT", "SOL/USDT"];

const MOCK_LUNARCRUSH_DATA = [
  { symbol: "BTC", alt_rank: 3, galaxy_score: 90 },
  { symbol: "ETH", alt_rank: 1, galaxy_score: 95 },
  { symbol: "DOGE", alt_rank: 2, galaxy_score: 85 },
  { symbol: "SOL", alt_rank: 4, galaxy_score: 80 },
];

beforeEach(() => {
  mock.reset();
});

describe("GET /fetchPairs/:exchange", () => {
  it("should sort LunarCrush data correctly based on lunarMode", async () => {
    // Mock LunarCrush API response
    mock.onGet("https://lunarcrush.com/api4/public/coins/list/v1").reply(200, {
      data: MOCK_LUNARCRUSH_DATA,
    });

    // Mock exchange data retrieval
    jest
      .spyOn(global, "fetchExchangePairs")
      .mockResolvedValue(MOCK_EXCHANGE_PAIRS);

    const response = await request(app).get(
      "/fetchPairs/binance?quoteAsset=USDT&lunarMode=alt_rank"
    );

    expect(response.status).toBe(200);
    expect(response.body.pairs).toEqual([
      "ETH/USDT",
      "DOGE/USDT",
      "BTC/USDT",
      "SOL/USDT",
    ]);
  });

  it("should filter pairs correctly based on available exchange pairs", async () => {
    mock.onGet("https://lunarcrush.com/api4/public/coins/list/v1").reply(200, {
      data: MOCK_LUNARCRUSH_DATA,
    });

    jest
      .spyOn(global, "fetchExchangePairs")
      .mockResolvedValue(["BTC/USDT", "ETH/USDT"]);

    const response = await request(app).get(
      "/fetchPairs/binance?quoteAsset=USDT&lunarMode=alt_rank"
    );

    expect(response.status).toBe(200);
    expect(response.body.pairs).toEqual(["ETH/USDT", "BTC/USDT"]);
  });

  it("should return an empty array if there are no matching pairs", async () => {
    mock.onGet("https://lunarcrush.com/api4/public/coins/list/v1").reply(200, {
      data: MOCK_LUNARCRUSH_DATA,
    });

    jest.spyOn(global, "fetchExchangePairs").mockResolvedValue(["XRP/USDT"]);

    const response = await request(app).get(
      "/fetchPairs/binance?quoteAsset=USDT&lunarMode=alt_rank"
    );

    expect(response.status).toBe(200);
    expect(response.body.pairs).toEqual([]);
  });

  it("should return an error if LunarCrush API fails", async () => {
    mock.onGet("https://lunarcrush.com/api4/public/coins/list/v1").reply(500);

    jest
      .spyOn(global, "fetchExchangePairs")
      .mockResolvedValue(MOCK_EXCHANGE_PAIRS);

    const response = await request(app).get("/fetchPairs/binance");

    expect(response.status).toBe(500);
    expect(response.body.error).toBe("Failed to fetch LunarCrush data");
  });
});
