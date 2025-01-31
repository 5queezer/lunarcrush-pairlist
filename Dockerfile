FROM oven/bun:latest
WORKDIR /app
COPY ./package.json ./package-lock.json ./
RUN bun install --production
COPY ./src/* .
ENV PORT=8080
EXPOSE 8080
CMD ["bun", "index.js"]
