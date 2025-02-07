import express from "express";
import cors from "cors";
import pairsRoutes from "@/routes/pairs.routes";
import { ENV } from "@/config/env";

const app = express();
app.use(cors());
app.use(express.json());
app.use(`${ENV.API_PREFIX || ""}/pairlist`, pairsRoutes);

const server = app.listen(ENV.PORT, () => console.log(`🚀 Server running on port ${ENV.PORT}`));

export { app, server }; // Export both app and server
