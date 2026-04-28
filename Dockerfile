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

# ── Stages 2/3 (pay + billing UI) deferred ──────────────────────────────
# Pay UI (hanzoai/pay) and billing UI (hanzoai/billing) are not yet
# tagged/published; their go:embed targets keep .gitkeep placeholders so
# the binary builds clean and serves an empty FS at /admin/pay and
# /admin/billing. Re-add the build stages once those repos ship a v*
# tag and overlay the bundles into checkout/ui/dist + billing/ui/dist
# from --from=pay-build / --from=billing-build like before.

# ── Stage 4: Build Go binary (with embedded admin + placeholder pay/billing) ──
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

# Pay UI + billing UI overlays deferred — see stages 2/3 comment above.
# The .gitkeep placeholders in checkout/ui/dist and billing/ui/dist keep
# go:embed happy; pay/billing endpoints serve an empty FS until the
# upstream repos ship a v* release.

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

# Default command. cmd/commerce/main.go is flag.Parse() only; positional
# args are ignored. Pass the listen address as a --http flag so the
# binary doesn't fall back to the 127.0.0.1:8090 default and CrashLoopBackOff
# at rollout. ENTRYPOINT + CMD form a single argv: /app/commerce --http 0.0.0.0:8001.
ENTRYPOINT ["/app/commerce"]
CMD ["--http", "0.0.0.0:8001"]
