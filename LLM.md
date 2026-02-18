# Commerce - LLM Context

## Overview

Multi-tenant e-commerce platform. Standalone Go binary with embedded SQLite, migrated from Google App Engine. Zero GAE dependencies remain.

**Live at**: https://commerce.hanzo.ai | **Version**: v1.33.0

## Architecture

```
Commerce App (Cobra CLI + Gin HTTP + Hooks + Events)
  |
  +-- User SQLite (data/users/{userID}/data.db) + sqlite-vec
  +-- Org SQLite (data/orgs/{orgID}/data.db) + sqlite-vec
  +-- PostgreSQL (alternative, pgvector)
  +-- MongoDB/FerretDB (alternative)
  +-- ClickHouse via hanzo/datastore-go (analytics)
```

## Multi-Tenancy

- Namespace-based: `Organization.Name` IS the namespace
- `middleware.Namespace()` sets appengine context namespace for downstream datastore
- `rest.New()` auto-applies namespace middleware unless `DefaultNamespace = true`
- Dual auth: legacy access token (org-bound) + IAM JWT (OIDC/JWKS via hanzo.id)
- `"platform"` org returns empty namespace (intentional admin bypass)

## Key Directories

```
commerce/
  cmd/commerce/    CLI entry point
  commerce.go      Main app framework
  db/              SQLite, Postgres, Mongo, ClickHouse backends
  hooks/           Hook system (Base-compatible): Hook[T], TaggedHook[T], Resolver
  events/          Unified event forwarding to ClickHouse/Insights/Analytics
  insights/        PostHog integration + Gin middleware
  api/             HTTP handlers (store, cart, analytics, namespace, etc.)
  models/          Data models
  middleware/      HTTP middleware (auth, namespace, IAM)
  infra/           Infrastructure clients (Redis, Meilisearch, etc.)
```

## Running

```bash
go run cmd/commerce/main.go serve --dev     # Development
./commerce serve 0.0.0.0:80                 # Production
```

## Environment Variables

| Variable | Default | Notes |
|----------|---------|-------|
| `COMMERCE_DIR` | `./commerce_data` | Data directory |
| `COMMERCE_SECRET` | `change-me-in-production` | Encryption secret |
| `COMMERCE_HTTP` | `127.0.0.1:8090` | Listen address |
| `REDIS_URL` | - | `redis://[:pass@]host:port[/db]` (priority over VALKEY_URL) |
| `COMMERCE_DATASTORE` | - | ClickHouse DSN |
| `INSIGHTS_ENABLED` | `false` | PostHog product analytics |
| `ANALYTICS_ENABLED` | `false` | Umami-like web analytics |
| `KMS_ENABLED` | `false` | Enable KMS secret management |
| `KMS_URL` | - | KMS base URL |
| `KMS_CLIENT_ID` | - | KMS Universal Auth client ID |
| `KMS_CLIENT_SECRET` | - | KMS Universal Auth client secret |
| `KMS_PROJECT_ID` | - | KMS project/workspace ID |
| `KMS_ENVIRONMENT` | `prod` | KMS environment |

## Analytics Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/analytics/event` | POST | Single event |
| `/api/v1/analytics/events` | POST | Batch events |
| `/api/v1/analytics/identify` | POST | User identification |
| `/api/v1/analytics/ast` | POST | astley.js page AST (JSON-LD) |
| `/api/v1/analytics/pixel.gif` | GET | Pixel tracking |
| `/api/v1/analytics/ai/message` | POST | AI message event |
| `/api/v1/analytics/ai/completion` | POST | AI completion event |

## Dependencies

**Core**: cobra, go-sqlite3, gin, hanzoai/datastore-go
**Infra**: go-redis/v9, minio-go/v7, meilisearch-go, nats.go, temporal SDK

## Security Audit (2026-02-14)

Fixed 6 multi-tenancy issues (all compile clean):

1. Namespace API had NO authentication -- added Admin token requirement
2. IAM middleware never resolved org from JWT `owner` claim -- now sets gin context
3. Store listing handlers used unscoped datastore -- added `orgNamespacedDB()` helper
4. Cart handlers used unscoped datastore -- changed to `datastore.New(org.Namespaced(c))`
5. Analytics trusted client-supplied org_id -- now overrides with authenticated org
6. `"platform"` org namespace bypass documented, `IsPlatformOrg()` helper added

## KMS Integration (2026-02-17)

Secrets management via KMS (Infisical-compatible REST API). KMS is the **single source of truth** for all payment provider credentials — no fallback to org-stored fields, no raw K8s secrets for payment providers.

**Architecture**: Credential Hydration (KMS-only, no fallback).

```
READ paths (hydration → org fields → downstream):
  checkout handlers → getOrganizationAndOrder() → kms.Hydrate() → org.StripeToken() etc.
  checkout sessions → Sessions() → kms.Hydrate() → org.StripeToken()
  subscriptions     → Subscribe/Update/Unsubscribe → hydrateOrg() → org.StripeToken()
  stripe webhooks   → Webhook() → getToken() + kms.Hydrate() → GetStripeAccessToken()

WRITE paths (credentials → KMS):
  seed command      → commerce seed → Client.SetSecret() for Stripe + Square
  stripe connect    → OAuth callback → org.Update() + Client.SetSecret()
  integration sync  → admin Upsert → org.Update() + Client.SetSecret()
```

**Hydration**: `kms.Hydrate(cc, org)` fetches all 25 provider credential fields from KMS and populates the org's integration struct fields. Called once after org resolution at every entry point. Missing secrets are silently skipped. The CachedClient's 5min TTL prevents repeated KMS calls.

**Secret path convention**:
```
/tenants/{orgName}/stripe/STRIPE_LIVE_ACCESS_TOKEN
/tenants/{orgName}/stripe/STRIPE_TEST_ACCESS_TOKEN
/tenants/{orgName}/stripe/STRIPE_PUBLISHABLE_KEY
/tenants/{orgName}/square/SQUARE_PRODUCTION_ACCESS_TOKEN (+ LOCATION_ID, APPLICATION_ID)
/tenants/{orgName}/square/SQUARE_SANDBOX_ACCESS_TOKEN (+ LOCATION_ID, APPLICATION_ID)
/tenants/{orgName}/authorizenet/AUTHORIZENET_LIVE_LOGIN_ID (+ TRANSACTION_KEY)
/tenants/{orgName}/authorizenet/AUTHORIZENET_SANDBOX_LOGIN_ID (+ TRANSACTION_KEY)
/tenants/{orgName}/paypal/PAYPAL_LIVE_* (EMAIL, SECURITY_USER_ID, SECURITY_PASSWORD, SECURITY_SIGNATURE, APPLICATION_ID)
/tenants/{orgName}/paypal/PAYPAL_TEST_* (same 5 fields)
```

**Write paths**: `commerce seed` writes env vars TO KMS. Stripe Connect OAuth callback writes tokens to KMS after exchange. Admin integration upsert syncs Stripe creds to KMS.

**Config**: `KMS_ENABLED`, `KMS_URL`, `KMS_CLIENT_ID`, `KMS_CLIENT_SECRET`, `KMS_PROJECT_ID`, `KMS_ENVIRONMENT`

**Cache**: 5min TTL, extends to 30min on KMS failure (stale-while-revalidate).

**K8s**: Single "secret zero" (`commerce-kms-auth`) holds KMS Universal Auth credentials. All payment credentials live in KMS only.

## Checkout Sessions (2026-02-17)

`POST /api/v1/checkout/sessions` — Public endpoint (no token auth) that creates Stripe Checkout Sessions.

**Request**: `{ company, providerHint, currency, org, customer, items, successUrl, cancelUrl }`
**Response**: `{ checkoutUrl, sessionId }`

Org resolved from `X-Hanzo-Org` header or request body `org`/`tenant` field. Per-request Stripe client (multi-tenant safe). Emits `checkout_started` analytics event.

## Gotchas

- Healthcheck: use `curl -f` not `wget --spider` (Gin only handles GET)
- Meilisearch v0.35.1 changed `AddDocuments`/`DeleteDocuments` signatures
- Production Dockerfile uses `CGO_ENABLED=0` for static binary
- Global entities (Organization, User, Token) use `DefaultNamespace = true` by design
