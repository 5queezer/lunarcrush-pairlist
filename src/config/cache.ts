import * as fs from "fs"; // Fix import issue
import { ENV } from "./env";

if (!fs.existsSync(ENV.CACHE_DIR)) {
  fs.mkdirSync(ENV.CACHE_DIR, { recursive: true });
}
