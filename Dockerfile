FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go test ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/keyantong-autosign .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /out/keyantong-autosign ./keyantong-autosign

ENV DATA_DIR=/app/data \
    HEALTH_CHECK_PORT=8080

VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/keyantong-autosign"]
