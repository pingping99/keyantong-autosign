FROM golang:1.21-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /src
COPY . .

RUN go mod tidy
RUN go build -o /app/signbot ./...

FROM alpine:3.21

RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /app/signbot ./signbot

ENTRYPOINT ["/app/signbot"]