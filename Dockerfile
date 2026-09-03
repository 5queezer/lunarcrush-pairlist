# syntax=docker/dockerfile:1.7
FROM golang:1.24.5-alpine3.22 AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY web ./web
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lunar-pairlist . \
    && mkdir -p /out/data \
    && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lunar-pairlist /app
COPY --from=build --chown=65532:65532 /out/data /data
ENV PORT=8080 DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD ["/app", "-healthcheck"]
USER nonroot:nonroot
ENTRYPOINT ["/app"]
