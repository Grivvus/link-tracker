FROM golang:1.26.2-alpine AS builder

WORKDIR /app

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY /migrations/ ./migrations/

ENTRYPOINT ["goose", "-env=none", "-dir=/app/migrations/", "up"]
