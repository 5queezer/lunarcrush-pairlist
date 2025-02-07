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
Object.defineProperty(exports, "__esModule", { value: true });
var express_1 = require("express");
var exchange_service_1 = require("../services/exchange.service");
var lunarcrush_service_1 = require("../services/lunarcrush.service");
var env_1 = require("../config/env");
var router = (0, express_1.Router)();
router.get("/lunar/:exchange/:marketType/:lunarMode", function (req, res) { return __awaiter(void 0, void 0, void 0, function () {
    var _a, exchange, marketTypeParam, lunarMode_1, marketType, limit, quoteAsset, _b, exchangePairs_1, lunarcrushData, sortedLunarcrush, lunarcrushCoins, intersection, error_1;
    return __generator(this, function (_c) {
        switch (_c.label) {
            case 0:
                _c.trys.push([0, 2, , 3]);
                _a = req.params, exchange = _a.exchange, marketTypeParam = _a.marketType, lunarMode_1 = _a.lunarMode;
                marketType = marketTypeParam;
                limit = parseInt(req.query.limit, 10) || 50;
                quoteAsset = req.query.quoteAsset || exchange == "hyperliquid"
                    ? "USDC"
                    : "USDT";
                return [4 /*yield*/, Promise.all([
                        (0, exchange_service_1.fetchExchangePairs)(exchange, quoteAsset, marketType),
                        (0, lunarcrush_service_1.fetchLunarcrushCoins)(),
                    ])];
            case 1:
                _b = _c.sent(), exchangePairs_1 = _b[0], lunarcrushData = _b[1];
                sortedLunarcrush = lunarcrushData.sort(function (a, b) {
                    return Number(a[lunarMode_1]) - Number(b[lunarMode_1]);
                });
                lunarcrushCoins = sortedLunarcrush.map(function (coin) { return coin.symbol; });
                intersection = lunarcrushCoins
                    .map(function (coin) {
                    return exchangePairs_1.find(function (pair) { return pair.startsWith("".concat(coin, "/")); }) || null;
                })
                    .filter(Boolean);
                res.json({
                    pairs: intersection.slice(0, limit),
                    refresh_period: env_1.ENV.CACHE_TTL_LUNARCRUSH,
                });
                return [3 /*break*/, 3];
            case 2:
                error_1 = _c.sent();
                res.status(500).json({ error: error_1.message });
                return [3 /*break*/, 3];
            case 3: return [2 /*return*/];
        }
    });
}); });
exports.default = router;
