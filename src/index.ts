import express from "express";
import cors from "cors";
import os from "os";
import fs from "fs";
import pairsRoutes from "@/routes/pairs.routes";
import healthRoutes from "@/routes/health.routes";
import { ENV } from "@/config/env";
import { cache, saveCacheToFile } from "@/utils/cacheHandler";

const app = express();
app.use(cors());
app.use(express.json());
app.use(`${ENV.API_PREFIX || ""}/pairlist`, pairsRoutes);
app.use(`${ENV.API_PREFIX || ""}/health`, healthRoutes);

// Detect if running inside Docker
const isDocker = () => {
  try {
    return (
      fs.existsSync("/.dockerenv") ||
      fs.readFileSync("/proc/self/cgroup", "utf8").includes("docker")
    );
  } catch {
    return false;
  }
};

const protocol = ENV.HTTPS ? "https" : "http";
const host = isDocker() ? os.hostname() : ENV.HOST || "localhost";
const port = ENV.PORT || 8080;
const baseUrl = `${protocol}://${host}:${port}${ENV.API_PREFIX || ""}`;

const server = app.listen(port, () =>
  console.log(`🚀 Server running at ${baseUrl}`)
);

// Handle process exit to save cache
const handleExit = (signal: NodeJS.Signals) => {
  console.log(`🛑 Received ${signal}. Saving cache and shutting down...`);
  saveCacheToFile(cache);
  process.exit();
};

process.on("SIGINT", handleExit);
process.on("SIGTERM", handleExit);

export { app, server, cache };
