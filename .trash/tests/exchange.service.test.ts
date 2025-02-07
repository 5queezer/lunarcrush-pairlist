import {
  fetchExchangePairs,
  MarketType,
} from "../src/services/exchange.service";
import * as ccxt from "ccxt";

jest.mock("ccxt");

const mockLoadMarkets = jest.fn().mockResolvedValue(undefined);

const mockMarkets = {
  BTCUSDT: { base: "BTC", quote: "USDT", type: "spot", active: true },
  ETHUSDT: { base: "ETH", quote: "USDT", type: "spot", active: true },
};

// Properly mock ccxt exchanges using `jest.spyOn`
jest.spyOn(ccxt, "binance").mockImplementation(
  () =>
    ({
      enableRateLimit: true,
      loadMarkets: mockLoadMarkets,
      markets: mockMarkets,
    } as any)
);

jest.spyOn(ccxt, "bybit").mockImplementation(
  () =>
    ({
      enableRateLimit: true,
      loadMarkets: mockLoadMarkets,
      markets: mockMarkets,
    } as any)
);

jest.spyOn(ccxt, "hyperliquid").mockImplementation(
  () =>
    ({
      enableRateLimit: true,
      loadMarkets: mockLoadMarkets,
      markets: mockMarkets,
    } as any)
);

describe("Exchange Service", () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  it("should fetch exchange pairs from Binance", async () => {
    const pairs = await fetchExchangePairs("binance", "USDT", MarketType.SPOT);
    expect(pairs).toEqual(["BTC/USDT", "ETH/USDT"]);
  });

  it("should throw an error for unsupported exchange", async () => {
    await expect(
      fetchExchangePairs("fakeExchange", "USDT", MarketType.SPOT)
    ).rejects.toThrow("Exchange fakeExchange is not supported.");
  });
});
