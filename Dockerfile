# syntax=docker/dockerfile:1
# Hanzo Commerce - E-commerce Platform
# Multi-stage build for minimal production image

# The two sibling UIs this binary embeds, named as the ARTIFACTS their own
# repos publish. A `FROM` in the first stage needs the ARG in global scope, so
# they live here; override either with --build-arg (or hanzo.yml `args:`).
ARG PAY_IMAGE=ghcr.io/hanzoai/commerce:pay-0.1.13@sha256:b25c5b6525e7b2bd71062aa63b552357d6ad76e7b9015e8e345d30c93dd24020
ARG BILLING_IMAGE=ghcr.io/hanzoai/billing:1.0.25

# ── Stage 1: THE Commerce admin, built from source ───────────────────────────
# app/admin (@hanzo/commerce-dashboard, Next.js `output: export`) on @hanzo/ui +
# @hanzo/gui — the same component set the cloud console renders. This stage IS
# the producer: it used to `COPY app/admin/out` out of the build context, which
# stopped existing the moment that export became gitignored build output, and it
# landed the copy in `admin/dist` — a path no go:embed has ever named — so the
# binary this Dockerfile ships carried no admin at all.
#
# @hanzo/ui comes from the registry at the same range the console pins, so this
# build and a developer's build resolve the identical package. It was a workspace
# link to a SIBLING checkout, which no single-repo build context can see: every
# image rewrote the manifest to the registry range first, and so built something
# no one had ever built locally — that is how a missing @hanzogui/next-theme
# reached a release.
FROM node:20-slim AS admin-build
RUN npm install -g pnpm@9.15.9
WORKDIR /build
# The FE pnpm workspace is the only input the admin build needs.
COPY app/ ./app/
WORKDIR /build/app
RUN pnpm install --frozen-lockfile

# Prod defaults baked into the client bundle (Next inlines NEXT_PUBLIC_*).
# commerce.hanzo.ai is same-origin for the API (bare /v1/*); hanzo.id is the OIDC
# issuer; client_id hanzo-commerce is the PKCE client; the AI dock calls the
# api.hanzo.ai chat-completions gateway.
ENV NEXT_PUBLIC_COMMERCE_API_URL=https://api.hanzo.ai \
    NEXT_PUBLIC_IAM_SERVER_URL=https://hanzo.id \
    NEXT_PUBLIC_IAM_CLIENT_ID=hanzo-commerce \
    NEXT_PUBLIC_HANZO_AI_URL=https://api.hanzo.ai/v1/chat/completions \
    NEXT_PUBLIC_HANZO_AI_MODEL=best

# typecheck is a real gate: the admin renders @hanzo/ui/product from source, so a
# drift in the shared component set fails HERE, not in a browser.
RUN node_modules/.bin/turbo run typecheck build --filter=@hanzo/commerce-dashboard

# ── Stage 2+3: the pay and billing UIs, TAKEN not REBUILT ────────────────────
# Both stages used to `git clone` their repo and run `pnpm build` — commerce
# rebuilding two other repos from source inside its own image. That needs a git
# credential for two PRIVATE repos in an org this build cannot see, and it
# didn't have one: the buildx `gh_token` secret is a real, working GitHub token
# (KMS hanzo /deploy GITHUB_TOKEN, user hanzo-dev), and hanzo-dev is not a
# member of hanzo-inc, so an AUTHENTICATED clone still got `Repository not
# found` — GitHub's 404 for "exists, not yours". No token that reaches this
# build can fix that; only a membership change could.
#
# But nothing here ever wanted pay's SOURCE. It wanted pay's dist/. And pay
# already builds and publishes that, on every push, through the same
# hanzoai/ci lane this repo uses — as does billing. So take the artifact.
# One producer per artifact, consumed where it is published:
#   ghcr.io/hanzoai/pay      /srv     ← the Vite SPA (was /pay/dist)
#   ghcr.io/hanzoai/billing  /public  ← the Next export (was /billing/out)
# Byte-identical to what the clone-and-build produced, minus two node
# toolchains and a pnpm install. The registry credential is one this build
# already holds — it is the same login that pushes ghcr.io/hanzoai/commerce a
# few minutes later — so this removes a credential requirement rather than
# adding one.
#
# Pinned to an immutable published semver, never `latest`: a mutable tag under
# a version name is how a rebuild silently changes what a release contains.
FROM ${PAY_IMAGE} AS pay-dist
FROM ${BILLING_IMAGE} AS billing-dist

# ── Stage 4: Build Go binary (with embedded admin + pay + billing SPAs) ──
FROM golang:1.26.5-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev

ARG TARGETARCH

WORKDIR /build

# Copy go mod files first for layer caching. thirdparty/ethereum (the LGPL-
# adjacent go-ethereum nested module) was ripped out; on-chain HUSD/ERC-20 now
# rides luxfi/cevm (Apache-2.0), a normal go-module dep resolved via the proxy —
# no nested-module go.mod/go.sum to COPY.
COPY go.mod go.sum ./

# NO GOPRIVATE. It used to say `github.com/hanzoai/*`, which routes every
# hanzoai module past the proxy and the checksum database, straight to git —
# and git is where three of them stopped existing. goauthorizenet, gochimp3
# and sendgrid-go moved to the same org pay and billing did, and now 404 on
# github.com for everyone, signed in or not. The other eighteen are plain
# public repos that never needed the exemption.
#
# All twenty-one are on proxy.golang.org at the exact version go.mod pins, and
# sum.golang.org returns the same h1: hashes go.sum already commits — so the
# proxy is not the loose path here, it is the VERIFIED one, and it holds an
# immutable copy of code whose repository is gone. `,direct` is still the
# fallback, still authenticated by the gh_token below.
#
# This build did not fail on it only because BuildKit's /go/pkg/mod cache
# mount was warm. That is the worst kind of green: the fleet scaled 10 → 20
# runners today, and every new one has a cold cache.
#
# GOTOOLCHAIN=local pins the builder's own golang:1.26.5 toolchain so go does
# NOT try to download+verify a toolchain module (which fails as a sumdb
# "SECURITY ERROR").
# GOWORK=off is critical: the repo commits a go.work (use . ./metering).
# Without disabling it, `go mod download` runs in workspace mode, reads the
# stale committed go.work.sum, and fails sum verification ("SECURITY ERROR").
# The image builds the single root module, so force module mode.
ENV GOTOOLCHAIN=local \
    GOWORK=off
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=secret,id=gh_token \
    if [ -s /run/secrets/gh_token ]; then \
      git config --global url."https://x-access-token:$(cat /run/secrets/gh_token)@github.com/".insteadOf "https://github.com/"; \
    fi; \
    go mod download

# Copy source code (note: api/billing/plans/ is gitignored — the pre-build
# step in the CI workflow curls the @hanzo/plans npm tarball into
# api/billing/plans/, so this COPY picks up the canonical SOT for
# go:embed at compile time. Local dev mirrors via scripts/fetch-plans.sh —
# public package, no auth needed).
COPY . .

# Populate api/billing/plans/*.json (gitignored, so absent from the git
# build context) from the public @hanzo/plans npm tarball. api/billing/
# plans.go go:embed's plans/dns.json + plans/subscription.json at compile
# time, so this MUST run before `go build`. No auth (public npm).
# Pin to the highest published version (the script's internal default tracked
# an unpublished tag and 404'd); override with --build-arg PLANS_VERSION=.
ARG PLANS_VERSION=1.1.4
RUN apk add --no-cache curl && sh scripts/fetch-plans.sh

# Bake the admin export into ui/dist for //go:embed (ui/embed.go). Same contract
# as the plans hydrate above: the directory is gitignored build output, so the ONE
# producer (scripts/sync-admin-ui.sh, fed by app/admin/out) must run before
# `go build`. bash + rsync are the script's only requirements.
COPY --from=admin-build /build/app/admin/out ./app/admin/out
RUN apk add --no-cache bash rsync && bash scripts/sync-admin-ui.sh

# Overlay the pay UI into checkout/ui/dist so go:embed in checkout/embed.go
# picks up the real SPA bundle.
RUN rm -rf checkout/ui/dist && mkdir -p checkout/ui/dist
COPY --from=pay-dist /srv/ checkout/ui/dist/

# Overlay the billing admin UI into billing/ui/dist so go:embed in
# billing/embed.go picks up the real Next.js export.
RUN rm -rf billing/ui/dist && mkdir -p billing/ui/dist
COPY --from=billing-dist /public/ billing/ui/dist/

# Build the binary with CGO for SQLite support.
#
# `-tags sqlite_math_functions` is REQUIRED for any cgo build: hanzoai/base's
# search layer generates SQL calling acos/cos/sin/radians/sqrt, and csqlite only
# compiles those in behind that tag. base asserts it at compile time
# (core/sqlite_math_required.go) rather than letting a cgo binary ship with an
# SQL surface smaller than the code written against it. Without the tag every
# build of this image fails on `undefined: cgoBuildNeedsSQLiteMathFunctions` —
# which is why v1.49.26 through v1.49.28 published no image at all.
#
# The HIP-0106 cloud-mount path is always compiled in — it costs one zip
# import, so there is nothing to gate. Legacy direct-Gin stays the default
# boot mode; operators flip --cloud or COMMERCE_MODE=cloud to serve commerce
# through a zip.App the way a host mounts it. See mount.go for the contract
# and cmd/commerce/main.go for the dispatcher.
#
# Pass `./cmd/commerce` (the package) instead of `./cmd/commerce/main.go`
# so cloud.go / legacy.go get compiled into the same binary alongside
# main.go.
# VERSION is injected by CI (docker-deploy.yml) from the immutable image
# tag so the binary's /healthz version == its deployed tag. Empty default
# keeps commerce.Version's in-source default for local builds.
ARG VERSION=""
# -mod=mod lets the Linux build complete go.sum itself (adding any go.mod
# hashes the build graph needs but that are absent from the committed go.sum).
# Private hanzoai modules are direct-git (GOPRIVATE) and their zip hashes differ
# by host OS, so go.sum cannot be regenerated reliably off-Linux — letting the
# build add the correct Linux hashes is the robust fix.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=gh_token \
    if [ -s /run/secrets/gh_token ]; then \
      git config --global url."https://x-access-token:$(cat /run/secrets/gh_token)@github.com/".insteadOf "https://github.com/"; \
    fi; \
    CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} \
    CGO_CFLAGS="-D_LARGEFILE64_SOURCE -D_GNU_SOURCE" \
    VER="${VERSION#v}" && \
    go build -mod=mod -p=8 \
    -tags "sqlite_omit_load_extension sqlite_math_functions" \
    -ldflags="-s -w \
      ${VER:+-X github.com/hanzoai/commerce.Version=${VER}} \
      -X github.com/hanzoai/commerce.GitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo sandboxfix) \
      -X github.com/hanzoai/commerce.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /build/commerce \
    ./cmd/commerce

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
