FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY config ./config
COPY core ./core

RUN CGO_ENABLED=0 go test ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/keyantong-autosign .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app \
    && mkdir -p /app/data \
    && chown -R app:app /app

WORKDIR /app
COPY --from=builder --chown=app:app /out/keyantong-autosign ./keyantong-autosign

ENV DATA_DIR=/app/data \
    HEALTH_CHECK_HOST=127.0.0.1 \
    HEALTH_CHECK_PORT=8080

VOLUME ["/app/data"]
EXPOSE 8080
USER app
ENTRYPOINT ["/app/keyantong-autosign"]
