FROM oven/bun:latest
WORKDIR /app
COPY ./package.json ./package-lock.json ./tsconfig.json ./
RUN bun install --production
COPY ./src src
ENV PORT=8080
EXPOSE ${PORT}
CMD ["bun", "run", "src/index.ts"]