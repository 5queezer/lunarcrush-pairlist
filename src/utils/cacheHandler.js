"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.cache = exports.saveCacheToFile = exports.loadCacheFromFile = void 0;
var fs_1 = require("fs");
var env_1 = require("../config/env");
var loadCacheFromFile = function () {
    try {
        if (fs_1.default.existsSync(env_1.ENV.CACHE_FILE_PATH)) {
            var data = fs_1.default.readFileSync(env_1.ENV.CACHE_FILE_PATH, "utf8");
            return JSON.parse(data);
        }
    }
    catch (error) {
        console.error("❌ Error loading cache from file:", error);
    }
    return { lunarcrush: null, exchanges: {} };
};
exports.loadCacheFromFile = loadCacheFromFile;
var saveCacheToFile = function (cache) {
    try {
        fs_1.default.writeFileSync(env_1.ENV.CACHE_FILE_PATH, JSON.stringify(cache, null, 2), "utf8");
        console.log("💾 Cache saved to file.");
    }
    catch (error) {
        console.error("❌ Error saving cache to file:", error);
    }
};
exports.saveCacheToFile = saveCacheToFile;
exports.cache = (0, exports.loadCacheFromFile)();
