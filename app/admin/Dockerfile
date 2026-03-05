# syntax=docker/dockerfile:1
# Hanzo Commerce Admin — Next.js standalone build
# Build context: commerce/app/ (the pnpm workspace root)
FROM node:22-alpine AS base

# ── Install ──────────────────────────────────────────────────────────────────
FROM base AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY admin/package.json admin/
COPY packages/ packages/
RUN corepack enable pnpm && pnpm install --frozen-lockfile

# ── Build ────────────────────────────────────────────────────────────────────
FROM base AS builder
WORKDIR /app

COPY --from=deps /app/ ./
COPY admin/ admin/
COPY packages/ packages/

WORKDIR /app/admin
RUN corepack enable pnpm && pnpm build

# ── Production ───────────────────────────────────────────────────────────────
FROM base AS runner

ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

RUN addgroup --system --gid 1001 nextjs && \
    adduser --system --uid 1001 nextjs

# standalone output puts server.js at the workspace root
WORKDIR /app/admin
COPY --from=builder --chown=nextjs:nextjs /app/admin/.next/standalone/ /app/
COPY --from=builder --chown=nextjs:nextjs /app/admin/.next/static ./.next/static

USER nextjs
EXPOSE 3000

CMD ["node", "server.js"]
