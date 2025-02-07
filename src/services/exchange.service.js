"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (g && (g = 0, op[0] && (_ = 0)), _) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
var __spreadArray = (this && this.__spreadArray) || function (to, from, pack) {
    if (pack || arguments.length === 2) for (var i = 0, l = from.length, ar; i < l; i++) {
        if (ar || !(i in from)) {
            if (!ar) ar = Array.prototype.slice.call(from, 0, i);
            ar[i] = from[i];
        }
    }
    return to.concat(ar || Array.prototype.slice.call(from));
};
var _a;
Object.defineProperty(exports, "__esModule", { value: true });
exports.fetchExchangePairs = exports.SUPPORTED_EXCHANGES = exports.CCXTMarketType = exports.MarketType = void 0;
var ccxt = require("ccxt");
var cacheHandler_1 = require("../utils/cacheHandler");
var env_1 = require("../config/env");
var MarketType;
(function (MarketType) {
    MarketType["SPOT"] = "spot";
    MarketType["FUTURES"] = "futures";
})(MarketType || (exports.MarketType = MarketType = {}));
exports.CCXTMarketType = (_a = {},
    _a[MarketType.SPOT] = "spot",
    _a[MarketType.FUTURES] = "swap",
    _a);
exports.SUPPORTED_EXCHANGES = {
    binance: new ccxt.binance({ enableRateLimit: true }),
    bybit: new ccxt.bybit({ enableRateLimit: true }),
    hyperliquid: new ccxt.hyperliquid({ enableRateLimit: true }),
};
var fetchExchangePairs = function (exchangeName_1) {
    var args_1 = [];
    for (var _i = 1; _i < arguments.length; _i++) {
        args_1[_i - 1] = arguments[_i];
    }
    return __awaiter(void 0, __spreadArray([exchangeName_1], args_1, true), void 0, function (exchangeName, quoteAsset, marketType) {
        var cacheKey, exchange, markets, pairs, error_1;
        if (quoteAsset === void 0) { quoteAsset = "USDT"; }
        if (marketType === void 0) { marketType = MarketType.SPOT; }
        return __generator(this, function (_a) {
            switch (_a.label) {
                case 0:
                    if (!exports.SUPPORTED_EXCHANGES[exchangeName]) {
                        throw new Error("Exchange ".concat(exchangeName, " is not supported."));
                    }
                    cacheKey = "".concat(exchangeName, "_").concat(marketType, "_").concat(quoteAsset);
                    if (cacheHandler_1.cache.exchanges[cacheKey] &&
                        Date.now() - cacheHandler_1.cache.exchanges[cacheKey].timestamp < env_1.ENV.CACHE_TTL_EXCHANGE) {
                        console.log("\u26A1 Returning cached ".concat(exchangeName, " data for ").concat(cacheKey));
                        return [2 /*return*/, cacheHandler_1.cache.exchanges[cacheKey].data];
                    }
                    _a.label = 1;
                case 1:
                    _a.trys.push([1, 3, , 4]);
                    exchange = exports.SUPPORTED_EXCHANGES[exchangeName];
                    return [4 /*yield*/, exchange.loadMarkets()];
                case 2:
                    _a.sent();
                    markets = Object.values(exchange.markets || {});
                    pairs = markets
                        .filter(function (market) {
                        return market &&
                            market.type === exports.CCXTMarketType[marketType] &&
                            market.quote === quoteAsset &&
                            market.active;
                    })
                        .map(function (market) { return market && "".concat(market.base, "/").concat(market.quote); })
                        .map(function (pair) {
                        return marketType === MarketType.FUTURES ? "".concat(pair, ":").concat(quoteAsset) : pair;
                    })
                        .filter(function (pair) { return !!pair; });
                    cacheHandler_1.cache.exchanges[cacheKey] = { data: pairs, timestamp: Date.now() };
                    return [2 /*return*/, pairs];
                case 3:
                    error_1 = _a.sent();
                    console.error("\u274C Error fetching ".concat(exchangeName, " pairs:"), error_1);
                    throw new Error("Failed to fetch ".concat(exchangeName, " data"));
                case 4: return [2 /*return*/];
            }
        });
    });
};
exports.fetchExchangePairs = fetchExchangePairs;
