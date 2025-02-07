import axios from "axios";
import { fetchLunarcrushCoins } from "../src/services/lunarcrush.service";
import { cache } from "../src/utils/cacheHandler";

jest.mock("axios");

describe("LunarCrush Service", () => {
  beforeEach(() => {
    cache.lunarcrush = null; // Reset cache before each test
    jest.clearAllMocks();
  });

  it("should fetch LunarCrush coins", async () => {
    const mockData = {
      data: { data: [{ symbol: "BTC" }, { symbol: "ETH" }] },
    };
    (axios.get as jest.Mock).mockResolvedValue(mockData);

    const coins = await fetchLunarcrushCoins();
    expect(coins).toEqual([{ symbol: "BTC" }, { symbol: "ETH" }]);
  });

  it("should throw an error when LunarCrush API fails", async () => {
    (axios.get as jest.Mock).mockRejectedValue(new Error("API error"));

    await expect(fetchLunarcrushCoins()).rejects.toThrow(
      "Failed to fetch LunarCrush data"
    );
  });
});
