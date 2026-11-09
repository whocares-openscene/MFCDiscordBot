FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache bash ca-certificates sqlite gcc musl-dev

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the bot
RUN CGO_ENABLED=0 go build -o mfcbot ./cmd/mfcbot

# Runtime stage
FROM alpine:3.22

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates

# Copy built binary from builder
COPY --from=builder /app/mfcbot /app/mfcbot

# Expose HTTP endpoints
EXPOSE 80

# Healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://localhost/healthz >/dev/null || exit 1

# Entrypoint
ENTRYPOINT ["./mfcbot"]