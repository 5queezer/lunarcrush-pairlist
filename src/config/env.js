"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.ENV = void 0;
var dotenv_1 = require("dotenv");
var path_1 = require("path");
dotenv_1.default.config();
exports.ENV = {
    PORT: process.env.PORT || "8080",
    BEARER_TOKEN: process.env.LUNARCRUSH_TOKEN || "",
    CACHE_DIR: process.env.CACHE_DIR || path_1.default.join(__dirname, "../../cache"),
    CACHE_FILE_PATH: path_1.default.join(process.env.CACHE_DIR || path_1.default.join(__dirname, "../../cache"), "cache.json"),
    CACHE_TTL_EXCHANGE: parseInt(process.env.CACHE_TTL_EXCHANGE || "0") / 1000 || 3600000, // 60 min
    CACHE_TTL_LUNARCRUSH: parseInt(process.env.CACHE_TTL_LUNARCRUSH || "0") / 1000 || 60000, // 1 min
    API_PREFIX: process.env.API_PREFIX || "",
};
if (!exports.ENV.BEARER_TOKEN)
    throw new Error("LunarCrush token not provided");
