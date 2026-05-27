# syntax=docker/dockerfile:1.7

FROM golang:1.24.5-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telegram-relay-bot ./cmd/bot && \
    mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
ENV DATABASE_PATH=/app/data/bot.db

COPY --from=builder --chown=nonroot:nonroot /out/telegram-relay-bot /app/telegram-relay-bot
COPY --from=builder --chown=nonroot:nonroot /out/data /app/data

VOLUME ["/app/data"]

USER nonroot:nonroot
ENTRYPOINT ["/app/telegram-relay-bot"]
