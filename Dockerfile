FROM oven/bun:latest
WORKDIR /app

# Copy package files first for caching
COPY package.json bun.lock ./
RUN bun install

# Copy the rest of the application
COPY . .

ENV PORT=8080
EXPOSE ${PORT}

CMD ["bun", "run", "src/index.ts"]
