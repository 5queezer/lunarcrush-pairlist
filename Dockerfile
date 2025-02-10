# Stage 1: Build
FROM oven/bun:latest AS builder
WORKDIR /app

# Copy package files first to leverage Docker cache
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy the rest of the application source code
COPY . .

# Build the project
RUN bun build ./src/index.ts --outdir ./dist --target node

# Stage 2: Production runtime
FROM oven/bun:latest AS runtime
WORKDIR /app

# Copy only necessary files from the builder stage
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/bun.lock ./bun.lock

ENV PORT=8080
EXPOSE ${PORT}

CMD ["bun", "run", "dist/index.js"]
