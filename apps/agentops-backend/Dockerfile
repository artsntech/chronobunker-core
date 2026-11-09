# --- Build stage ---
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy Go module files and download dependencies
COPY go.mod go.sum* ./
RUN go mod download

# Copy the backend source code
COPY . .

# Build static Linux amd64 binaries for API, Worker, and CLI
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/chronobunker-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/chronobunker-worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /build/chronobunker-cli ./cmd/cli

# --- Runtime stage ---
FROM alpine:3.19

# Install bash, openssl (for key generation), psql client (for migrations), and TLS certs
RUN apk add --no-cache bash openssl postgresql16-client ca-certificates && \
    adduser -D -u 10001 chrono

WORKDIR /app

# Copy compiled binaries
COPY --from=builder /build/chronobunker-api /app/chronobunker-api
COPY --from=builder /build/chronobunker-worker /app/chronobunker-worker
COPY --from=builder /build/chronobunker-cli /app/chronobunker-cli

# Prepare directories with proper ownership
RUN mkdir -p /app/secrets /app/policies /app/scripts /app/db && \
    chown -R chrono:chrono /app

USER 10001

EXPOSE 8080 9090

ENTRYPOINT ["/app/chronobunker-api"]