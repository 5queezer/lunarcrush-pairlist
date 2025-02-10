FROM oven/bun:latest
WORKDIR /app

# Copy only necessary files first for better caching
COPY package.json bun.lock ./
RUN bun install --production

# Copy the rest of the project
COPY tsconfig.json ./
COPY src ./src

ENV PORT=8080
EXPOSE ${PORT}

CMD ["bun", "run", "src/index.ts"]
