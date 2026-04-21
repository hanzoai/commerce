# syntax=docker/dockerfile:1
# Hanzo Commerce - E-commerce Platform
# Multi-stage build for minimal production image

# ── Stage 1: Build admin SPA (Next.js static export) ─────────────────────
FROM node:22-alpine AS admin-build
WORKDIR /web
RUN apk add --no-cache libc6-compat && corepack enable pnpm
COPY app/package.json app/pnpm-lock.yaml app/pnpm-workspace.yaml ./
COPY app/admin/package.json admin/
COPY app/packages/ packages/
RUN pnpm install --frozen-lockfile
COPY app/admin/ admin/
WORKDIR /web/admin
RUN pnpm build

# ── Stage 2: Build pay UI (Vite SPA from hanzoai/pay) ────────────────────
# Canonical source lives at github.com/hanzoai/pay. Forks override PAY_REPO
# and PAY_VERSION via --build-arg; default tracks the latest tagged release.
FROM node:22-alpine AS pay-build
ARG PAY_REPO=https://github.com/hanzoai/pay.git
ARG PAY_VERSION=v0.1.0
WORKDIR /pay
RUN apk add --no-cache git && corepack enable pnpm
RUN git clone --depth=1 --branch=${PAY_VERSION} ${PAY_REPO} /pay
RUN pnpm install --frozen-lockfile && pnpm build

# ── Stage 3: Build Go binary (with embedded admin + pay SPA) ─────────────
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

ARG TARGETARCH

WORKDIR /build

# Copy go mod files first for layer caching
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Replace placeholder dist/ with the real Next.js export so go:embed picks up
# the actual SPA bundle at compile time.
RUN rm -rf admin/dist
COPY --from=admin-build /web/admin/out admin/dist

# Overlay the pay UI build into checkout/ui/dist so go:embed in
# checkout/embed.go picks up the real SPA bundle.
RUN rm -rf checkout/ui/dist && mkdir -p checkout/ui/dist
COPY --from=pay-build /pay/dist/ checkout/ui/dist/

# Build the binary with CGO for SQLite support
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOMAXPROCS=1 GOOS=linux GOARCH=${TARGETARCH} go build -p=1 \
    -ldflags="-s -w \
      -X github.com/hanzoai/commerce.GitCommit=$(git rev-parse --short HEAD) \
      -X github.com/hanzoai/commerce.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /build/commerce \
    ./cmd/commerce/main.go

# Production stage
FROM alpine:3.21

LABEL org.opencontainers.image.source="https://github.com/hanzoai/commerce"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -S hanzo && adduser -S hanzo -G hanzo

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/commerce /app/commerce

# Copy templates and static assets
COPY --from=builder /build/templates /app/templates
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
    CMD curl -f http://localhost:8001/healthz || exit 1

# Default command
ENTRYPOINT ["/app/commerce"]
CMD ["serve", "0.0.0.0:8001"]
