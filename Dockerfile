# Hanzo Commerce - E-commerce Platform
# Multi-stage build for minimal production image

# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO for SQLite support
ARG TARGETARCH
RUN CGO_ENABLED=1 GOMAXPROCS=1 GOOS=linux GOARCH=${TARGETARCH} go build -p=1 \
    -ldflags="-s -w" \
    -o /build/commerce \
    ./cmd/commerce/main.go

# Production stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -S hanzo && adduser -S hanzo -G hanzo

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/commerce /app/commerce

# Copy templates and static assets
COPY --from=builder /build/templates /app/templates
COPY --from=builder /build/analytics/templates /app/analytics/templates
COPY --from=builder /build/api/templates /app/api/templates

# Create data directories
RUN mkdir -p /app/data /app/logs && \
    chown -R hanzo:hanzo /app

USER hanzo

# Expose default port
EXPOSE 8001

# Environment variables
ENV COMMERCE_DIR=/app/data
ENV COMMERCE_DEV=false
ENV PORT=8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8001/health || exit 1

# Default command
ENTRYPOINT ["/app/commerce"]
CMD ["serve", "0.0.0.0:8001"]
