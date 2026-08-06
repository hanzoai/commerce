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
  +-- ClickHouse via hanzo/datastore-go (analytics)
```

## Commerce Admin

There is ONE Commerce admin: the Next static export in `app/admin`
(`@hanzo/commerce-dashboard`), embedded in the Go binary (`ui/dist`,
`//go:embed`) and served by it at **`/admin`**. It renders **`@hanzo/ui/product`**
— the same component set the cloud console renders (`DataTable`, `PageHeader`,
`StatusTag`, `EmptyState`, `MetricCard`, `BackendStateCard`, `CommerceResource`)
— on `@hanzo/gui`. There is no commerce-local design system for the admin, and no
second shell.

**One artifact, one URL contract.** `next.config.ts` sets `basePath: '/admin'`
(from `src/lib/basepath.ts`), so chunks are `/admin/_next/*` and the client
router resolves routes under the same prefix. A `basePath`-less export is built
for `/` and CANNOT be served from a sub-path: its root-absolute `/_next/*`
escapes the mount and its router 404s on its own pathname. That is why the
separate `commerce-admin` static-container image is gone — it served the same
bundle at a second origin's ROOT, i.e. one artifact with two incompatible URL
contracts, and keeping it is what let the embedded copy rot.

The other three spellings are gone: the Vite SPA at `frontend/` (deleted — its
`@hanzogui/admin` `file:` target no longer exists), `hanzoai/admin`
`apps/admin-commerce` (retired), and the dead Medusa React-Router tree under
`app/admin/src/routes` (deleted along with the vendored Tailwind surface).

- **The catalog is data.** `src/lib/resources.ts` is a plain list of rows —
  slug, backend noun, copy, column specs. `src/lib/columns.tsx` turns a column
  spec into a rendered column; `src/app/(dashboard)/[resource]/page.tsx` emits one
  static page per row via `generateStaticParams`, and the sidebar reads the same
  list. Adding a merchant surface is adding a ROW, never a directory. The module is
  deliberately component-free: `generateStaticParams` runs it in Node at build time.
- **One client.** `src/lib/commerce.ts` calls the bare `/v1/<kind>` resources
  (`/v1/product`, `/v1/order`, `/v1/c/user`, `/v1/collection`, `/v1/stocklocation`,
  `/v1/store/current`) on `NEXT_PUBLIC_COMMERCE_API_URL` with the IAM bearer and
  `X-Org-Id`. Failures THROW with their status, so `classifyBackend` renders the
  honest state (402 → add credits, 403 → not enabled, 404 → not mounted) instead of
  a fabricated empty table.
- **Effects are injected.** `@hanzo/ui/product` never imports a router or an auth
  module; `src/app/providers.tsx` supplies `signIn`/`addCredits` once through
  `HostProvider`.
- IAM is native PKCE through `@hanzo/iam`; the active organization scopes every
  read and is the sole tenant selector.
- **`@hanzo/ui` comes from the registry at the range the console pins**
  (`^8.0.12`) so the admin and the console cannot drift, and so a developer's
  build and an image build resolve the identical package. It was a `link:` to a
  sibling checkout that every image rewrote to the registry range before
  installing — an image built what nobody had built locally, which is how a
  missing `@hanzogui/next-theme` (imported by `@hanzo/ui/product`'s ThemeToggle,
  declared by `@hanzo/ui` only as a devDependency) reached a release and failed
  `next build` there and only there.
- **`@hanzo/ui` declares none of the singletons it imports, so webpack must be
  told.** Each consumer installs its own `@hanzo/gui` / `@hanzogui/*` and
  resolution is free to pick a transitive copy. The bundle then holds two `@hanzogui/web`s — two
  `ThemeStateContext`s — `GuiProvider` publishes to one, every shared component
  reads the other, and the admin dies on load with `Missing theme.` `tsconfig`
  `paths` already pinned this for the TYPE layer; `next.config.ts`'s `SINGLETONS`
  alias now pins the RUNTIME layer to match. Symptom of a regression here: a
  blank admin with "Application error: a client-side exception".
- **The build is a real gate**: `turbo run typecheck build --filter=@hanzo/commerce-dashboard`.
  `next.config.ts` no longer sets `ignoreBuildErrors`, and `hanzo.yml` runs the
  typecheck in CI.
- `@hanzo/commerce-ui` (`app/packages/ui`, the vendored Medusa design system) is now
  a STOREFRONT-only dependency — `store/` still renders it. The admin is fully off it.
- The embedded copy in the Go binary is the SAME export: `scripts/sync-admin-ui.sh`
  copies `app/admin/out` into `ui/dist` for `//go:embed`, served at the `/admin/*`
  mount. `ui/dist` is BUILD OUTPUT — gitignored but for `.gitkeep`, and that sync is
  its ONE producer. It was tracked content until it shipped a retired Vite SPA:
  a committed bundle is a bundle nobody rebuilds. Run the producer before any
  `go build`, which is exactly what `Dockerfile` does.
- A sub-path mount is not a constraint on the export, it is a PARAMETER of it: an
  export can be served from `/admin` precisely because it was BUILT for `/admin`
  (`basePath`). What cannot be served there is an export built for `/` — its
  root-absolute `/_next/*` escapes the mount. `TestEmbedAdminServesItsOwnAssets`
  asserts this, on content type: the static fallback answers every miss with
  index.html and a 200, so status alone cannot see the break.
- `Dockerfile.store` builds the storefront and its workspace packages through the
  root pnpm/Turbo graph, then publishes the standalone server. Do not copy a partial
  package set or introduce a second package manager.

## Three credentials, three distinct jobs — do NOT collapse them

An audit will read commerce as having "three competing auth systems". It has one
identity system and two machine credentials, and they are different VALUES, not
three ways to spell one:

- **IAM / OIDC** — human identity. The only source of user identity. A validated
  principal is proven by `X-User-Id`, which is minted solely from a verified
  credential and never restored from client input.
- **`COMMERCE_SERVICE_TOKEN`** — ONE global platform machine identity (the
  cloud→commerce S2S dispatch). Checked FIRST in `TokenRequired`, ahead of the IAM
  branch, so an S2S call never meets the per-user scope gate.
- **Published storefront token** — a PER-ORG machine credential, minted by an org
  admin via `POST /v1/store/token` (`org.AddToken("storefront",
  permission.Published)`) and consumed as `HANZO_COMMERCE_STOREFRONT_TOKEN` by the
  storefront BFF. Org-bound, scoped, revocable by re-minting.

That third one rides the legacy per-org access-token path in
`middleware/accesstoken.go` — `GetWithAccessToken` → `tok.Verify(org.SecretKey)` →
`tok.Permissions` → `org.Live`. It LOOKS like dead duplication and is not: delete
it and every storefront's checkout stops authenticating. IAM does not currently
issue per-org, org-admin-mintable, revocable scoped keys, so folding this into IAM
is a migration that needs that capability first — not a cleanup.

## Multi-Tenancy

- Namespace-based: `Organization.Name` IS the namespace
- `middleware.Namespace()` sets context namespace for downstream datastore
- `rest.New()` auto-applies namespace middleware unless `DefaultNamespace = true`
- Dual auth: legacy access token (org-bound) + IAM JWT (OIDC/JWKS via hanzo.id)
- Every org (incl. `"platform"`) is STRICTLY scoped to its own name. The legacy
  `"platform" -> "" (cross-org) namespace` bypass was REMOVED (1.42.40, Red M1):
  it keyed cross-org datastore on an org-NAME string, not real platform-admin
  identity. Cross-org access gates on `auth.IAMClaims.GlobalAdmin()` ONLY.

## Gateway Trust Headers (2026-04-27)

commerced does NOT validate JWTs in-binary. The trust boundary is hanzoai/gateway.
Only the gateway-minted X-* identity headers are trusted. Headers are stripped
unconditionally on ingress to prevent client spoofing (see gateway/auth_middleware.go
stripIdentityHeaders).

**Directly-exposed edge (`COMMERCE_EDGE_AUTH=true`) — boundary mount point (Red CRITICAL, 1.42.40):**
commerce-api is reachable in-cluster at `commerce.hanzo.svc:8001` WITHOUT the
gateway, so it strips + re-mints identity itself via `middleware.EdgeAuth`. That
boundary MUST be installed before any route group: gin applies `engine.Use()`
only to routes registered AFTER the call. It is now installed in `Bootstrap`
(`server.go installIdentityBoundary`, called before `setupRoutes`), so it covers
`/_/commerce/*`, `/v1/commerce/*` AND the post-Bootstrap `/v1` `api.Route()`
bundle. Previously it was mounted from `embed.go` AFTER Bootstrap, so gin left
the setupRoutes groups unguarded and an in-cluster pod could `POST
/_/commerce/tenants` with forged `X-Org-Id: admin` + `X-User-IsGlobalAdmin: true`
→ 201 (platform superadmin by header forgery). The boundary NEVER 401s opaque
service tokens (not JWTs) and does NOT strip `X-Hanzo-Org`, so the cloud-api →
commerce per-org billing money path is untouched (`require=false`;
`COMMERCED_REQUIRE_IDENTITY` is incompatible with the no-X-Org-Id service-token
path). Regression: `edgeauth_standalone_test.go`, `middleware/edgeauth_test.go`.

| Header                | Source                  | Use                                         |
|-----------------------|-------------------------|---------------------------------------------|
| X-Org-Id              | JWT `owner` claim       | Org slug; namespace + scope                 |
| X-User-Id             | JWT `sub` claim         | User identity                               |
| X-User-Email          | JWT `email` claim       | Email; audit + notifications                |
| X-User-IsAdmin        | JWT `isAdmin` claim     | "true" iff ORG-level admin (an org owner)   |
| X-User-IsGlobalAdmin  | gateway-derived         | "true" iff PLATFORM (global) admin          |
| X-Roles               | JWT `roles` claim       | Comma-joined role names (admin/owner/etc.)  |
| X-User-Permissions    | gateway-derived         | bit.Field as base-10 int; 0 fails closed    |

Fail-closed contract: missing X-User-IsAdmin -> IsAdmin=false; missing
X-User-IsGlobalAdmin -> IsGlobalAdmin=false. Missing X-User-Permissions ->
bit.Field(0). Identity headers absent -> handler chain falls through to legacy
auth (or 401 when COMMERCED_REQUIRE_IDENTITY).

**Org-admin vs global-admin (Red — anti-conflation):** `X-User-IsAdmin` is the
ORG-level admin role — an org owner (e.g. `maxpower`) carries `isAdmin=true`
within their own org. It is ONLY for org-scoped RBAC. Cross-org / superadmin
actions (e.g. POST `/_/commerce/tenants`) MUST gate on
`auth.IAMClaims.GlobalAdmin()` — the explicit `isGlobalAdmin` claim
(X-User-IsGlobalAdmin) OR `owner=="admin"` — NEVER on `IsAdmin` nor an
org-mintable role NAME like "superadmin". `iammiddleware.GetIAMClaims` populates
both `IsAdmin` (X-User-IsAdmin) and `IsGlobalAdmin` (X-User-IsGlobalAdmin); the
gateway mints X-User-IsGlobalAdmin only for a real global admin and strips it on
ingress, so it can't be forged. Predicates: `checkout.isSuperadmin` =
`GlobalAdmin()`; `checkout.isTenantAdmin` = the robust org-level `IsAdmin` claim
(not a role string). Tests: `auth/globaladmin_test.go`,
`checkout/admin_tenants_authz_test.go`, `middleware/edgeauth_test.go`.

### EdgeAuth admin billing-view override (middleware/edgeauth.go, 1.42.36+)

At the standalone edge (COMMERCE_EDGE_AUTH=true) EdgeAuth normally locks every
`/billing/` request to the caller's OWN org (X-Org-Id + user/userId/customerId
== claims.Owner) — strict per-org isolation. A GLOBAL ADMIN may retarget the
view to another org via `?org=<slug>`: `resolveBillingSubject()` sets both the
namespace (X-Org-Id) and the locked subject to the requested org. The override
is HONORED only when `isGlobalAdmin(claims)` holds — `claims.IsGlobalAdmin` OR
`claims.Owner=="admin"` (NOT plain `IsAdmin`, which is an org-level role: an org
owner like maxpower has it). For everyone else the `?org` param is
consumed-and-ignored (stripped, never honored) so isolation can never be
weakened. Tests: middleware/edgeauth_test.go (admin-switch, non-admin-isolation,
bad-slug reject). `auth.IAMClaims` carries `IsGlobalAdmin` (json `isGlobalAdmin`).

Call sites read identity via:
- `pkg/auth.OrgID(ctx)` / `UserID(ctx)` / `UserEmail(ctx)` (preferred)
- `iammiddleware.GetIAMClaims(c)` returns a real `*auth.IAMClaims` populated from
  headers (Owner=X-Org-Id, Subject=X-User-Id, Name=X-User-Id, Email=X-User-Email,
  IsAdmin=X-User-IsAdmin=="true", Roles split from X-Roles). Never nil; do not
  add nil-guards downstream.

## Key Directories

```
commerce/
  cmd/commerce/    CLI entry point
  commerce.go      Main app framework
  db/              SQLite, Postgres, ClickHouse backends
  hooks/           Hook system (Base-compatible): Hook[T], TaggedHook[T], Resolver
  events/          Unified event forwarding to ClickHouse/Insights/Analytics
  insights/        Hanzo Insights integration + Gin middleware
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
| `INSIGHTS_ENABLED` | `false` | Hanzo Insights product analytics |
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
| `/v1/analytics/event` | POST | Single event |
| `/v1/analytics/events` | POST | Batch events |
| `/v1/analytics/identify` | POST | User identification |
| `/v1/analytics/ast` | POST | astley.js page AST (JSON-LD) |
| `/v1/analytics/pixel.gif` | GET | Pixel tracking |
| `/v1/analytics/ai/message` | POST | AI message event |
| `/v1/analytics/ai/completion` | POST | AI completion event |

## SaaS Metrics God-View (api/metrics, 2026-07)

`GET /v1/commerce/metrics/saas` — the whole-business SaaS operations snapshot for
admin.hanzo.ai's "SaaS Metrics" board. ONE cross-org walk (proven all-orgs pattern
from `api/billing.RunBillingCycleAllOrgs`), cached 45s, folding every org's
**subscription** ledger (`models/subscription`) + **api-usage** transaction ledger
into: revenue (MRR/ARR, new/churned MRR, MRR by plan category from the embedded
@hanzo/plans catalog), subscription mix (per-plan active/trialing/seats + a recent
create/cancel events feed), metered pay-as-you-go revenue totals, and top customers.
Params: `window` (7d/30d/90d/mtd/all), `limit` (customers cap). Bundle-child subs
(ProviderType "bundle") are excluded from counts/MRR. Upgrades/downgrades are `null`
(plan-change events are not written to `models/billingevent`).

Gate: `middleware.RequirePlatformAdmin` — the SINGLE cross-org read gate (global
admin OR trusted service token; org-admin refused). `api/costs.requireCostsAdmin`
delegates to it, so the two god-views share ONE gate definition. The route-level
`TokenRequired(permission.Admin)` is a no-op on the IAM path — the in-handler gate
is the boundary. The console reaches it through its OWN global-admin-gated proxy
(`app/admin/saas`) forwarding `COMMERCE_SERVICE_TOKEN`.

Deliberately NOT here (owned by the fleet o11y god-view `GET /v1/admin/o11y`):
per-model LLM tokens/latency/error and the per-org AI-spend ranking. Commerce names
those in the response `gaps[]` array, never fabricates. Per-model latency is
captured nowhere in the stack; per-model error rate is in ClickHouse
`cloud_usage.status` but not yet surfaced by the o11y `topModels` SQL.

## Dependencies

**Core**: cobra, go-sqlite3, gin, hanzoai/datastore-go
**Infra**: go-redis/v9, hanzos3/go (the house S3 client — package `minio`, republishes storage-go under the s3-go import path), meilisearch-go, nats.go, temporal SDK
**Vector**: Qdrant via REST/HTTP (port 6333). No gRPC, no vector-go SDK -- plain net/http + encoding/json.

## Security Audit (2026-02-14)

Fixed 6 multi-tenancy issues (all compile clean):

1. Namespace API had NO authentication -- added Admin token requirement
2. IAM middleware never resolved org from JWT `owner` claim -- now sets gin context
3. Store listing handlers used unscoped datastore -- added `orgNamespacedDB()` helper
4. Cart handlers used unscoped datastore -- changed to `datastore.New(org.Namespaced(c))`
5. Analytics trusted client-supplied org_id -- now overrides with authenticated org
6. `"platform"` org namespace bypass documented, `IsPlatformOrg()` helper added

## KMS Integration (2026-02-17)

Secrets management via KMS (REST API). KMS is the **single source of truth** for all payment provider credentials — no fallback to org-stored fields, no raw K8s secrets for payment providers.

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

## Checkout Sessions

`POST /v1/checkout/sessions` — AUTHENTICATED hosted-checkout mint (Square Payment
Link). Same `publishedRequired` gate as every sibling checkout route
(`TokenRequired(Admin, Published)`): a service token, a per-org **Published**
storefront token, or an IAM principal. **There is no anonymous path** — an
unauthenticated POST is 401 BEFORE any org resolution or Square call (v1.46.4
closed the anon mint hole; the earlier "public, no token auth" design was the
CRIT vulnerability).

**v1.46.5 closed the opaque-bearer bypass (CRIT, two layers).** RED live-proved
that `Authorization: Bearer <any-opaque-non-JWT-string>` + `X-Org-Id: <target>`
minted for the attacker's chosen org: EdgeAuth *restored* the client X-Org-Id for
any opaque bearer (before the token was validated) → `IAMTokenRequired` trusted
the header (set `iam_authenticated`, no token check) → `TokenRequired`'s IAM
branch was a bare `c.Next()` (no mask check). Fixed at BOTH layers, each of which
independently kills it:
1. **EdgeAuth (`middleware/edgeauth.go`)** — an opaque bearer's client X-Org-Id is
   NEVER restored to the trusted header; it is stashed in a PRIVATE ctx key
   (`ctxKeyClientOrg`) that ONLY `TokenRequired`'s service-token branch reads,
   AFTER verifying the bearer == `COMMERCE_SERVICE_TOKEN`. X-Org-Id stays
   stripped, so an unvalidated token can never resolve an org via
   `IAMTokenRequired`. The service-token branch reads that key (then falls back
   to the raw X-Org-Id header for gateway/EdgeAuth-off deployments), so per-org
   billing is unchanged.
2. **`middleware/accesstoken.go` `TokenRequired`** — the IAM branch now ENFORCES
   the requested masks (`len(masks)==0 || hasScope(c, permissions)`), so a
   forged/low-priv IAM principal (perms=0) is denied a `TokenRequired(Admin,
   Published)` mint (403). No-mask gates (billing) still admit any authenticated
   principal. Plus `iammiddleware.IsIAMAuthenticated` no longer falls back to raw
   `X-Org-Id` header presence — only the validated `iam_authenticated` gin key
   counts. Regression: `middleware/edgeauth_test.go`,
   `middleware/accesstoken_test.go`, `middleware/iammiddleware/authz_test.go`.

**v1.46.6 — Red re-review hardening (no bypass reopened).** Red confirmed 1.46.5
fully closes the bypass (SHIP); 1.46.6 addresses its LOW+INFO findings:
`checkout/wire.Instructions` now uses `GetOrganizationOK` and fails CLOSED with
401 (was a `GetOrganization` MustGet panic → 500 on any org-less call after
EdgeAuth strips X-Org-Id; also never leaks an org's bank/routing/SWIFT to an
unauthenticated caller — test `api/checkout/wire/instructions_test.go`);
`oauthmiddleware.TokenRequired`'s (currently unwired) IAM branch converged to the
same mask enforcement; stale `api/billing/handlers.go` comment corrected.

⚠️ **CORRECTION (2026-07-26): `COMMERCE_EDGE_AUTH` is set in NO deployment, so
Layer 1 has never been live.** This line used to read "Keep
`COMMERCE_EDGE_AUTH=true` pinned — the Layer-1 defense depends on it", which read
as a description of the running system. It is not: `git grep COMMERCE_EDGE_AUTH`
returns zero hits in hanzoai/cloud and zero in universe's `infra/k8s`. Proven from
outside, not just by grep — EdgeAuth's Layer 1 *deletes* a client `X-Org-Id`, yet
`GET /v1/store/current` with a forged `X-Org-Id` and no token returned 200 on
**both** api.hanzo.ai and commerce.hanzo.ai. That is only possible with EdgeAuth
off on both.

So of the two "independent" layers, only Layer 2 (the mask enforcement in
`TokenRequired`) was ever running — and it does not cover no-mask routes, which is
exactly how `/v1/store/{current,access}` leaked another org's store record to an
unauthenticated caller. The layer that actually holds now is
`IAMTokenRequired` requiring a validated principal (X-User-Id), not merely an
X-Org-Id selector — same predicate as cloud's `clients/principal.Validated`. It
needs no env flag, which is the point: a defense whose "on" switch is set nowhere
is not a defense.

Before relying on EdgeAuth for anything, check it is actually enabled where you
think it is. Deciding whether it survives at all belongs with the standalone
question below.

**Request**: `{ company, providerHint, currency, customer, items, successUrl, cancelUrl, couponCode? }`
**Response**: `{ checkoutUrl, sessionId }`

Security invariants (do not regress):
- **One org source**: the org is SOLELY the authenticated principal's
  (`authedOrg(c)` = `middleware.GetOrganization` + KMS hydrate). There is NO
  request-body `org`/`tenant` field and NO `?org`/`X-Org-Id`/`X-IAM-Org` override
  on the mint path — a caller can only ever mint for its OWN tenant.
- **Fail-closed creds**: an org without its own per-tenant KMS Square creds
  cannot mint live — `squareCheckoutClientForOrg` has NO production env/platform
  fallback (`isPlatformOrg` removed). Only SANDBOX keeps the env fallback (no
  money at risk), behind auth, so the per-org sandbox storefront still mints.
- **Redirect allowlist**: `successUrl`/`cancelUrl` are bounded to the org's own
  domains via `checkout.AllowedCheckoutRedirect` (org `Websites` + brand
  first-party hosts + brand registrable domains) — closes the open-redirect /
  phishing pivot on the minted link. Enforced for authed callers too.
- Item prices are server-authoritative from the org's per-org catalog
  (`catalogPrice`); the client `unitPrice` is ignored. Emits `checkout.started`.

The legitimate storefront BFF (server-side, holds the service token or the
per-org Published storefront token — mint via `POST /v1/store/token`)
is the ONE reachable checkout entry. A browser must never call this endpoint
directly with a client-chosen org.

## SQLite Query Engine (2026-02-23)

**Critical: PascalCase→camelCase conversion**. The SQLite query builder converts Go struct
field names (PascalCase) to JSON field names (camelCase) automatically via `toJSONFieldName()`.
This is needed because `json.Marshal()` uses json struct tags (camelCase), but callers use Go
field names in `Filter()`/`Order()` calls (legacy from Cloud Datastore migration).

- `Filter("Test=", false)` → `json_extract(data, '$.test') = 0` (not `$.Test`)
- `Filter("DestinationKind=", kind)` → `json_extract(data, '$.destinationKind')`
- Nested paths handled: `Filter("Account.TransactionHash=", h)` → `$.account.transactionHash`
- Boolean false/0 handled via COALESCE: `COALESCE(json_extract(...), 0) = 0` (handles NULL from omitempty)

**Data directory**: `{COMMERCE_DIR}/orgs/{orgName}/data.db` per org. System data in `orgs/system/data.db`.

**Tenant store lifecycle lives in `orm/db.Registry`, not here.** `db.Manager`
used to hold a `userDBs` and an `orgDBs` map, opening a store per tenant ever
touched and closing them only at shutdown — descriptors and memory grew with the
tenant count, with no bound and no replication. It now delegates to
`ormdb.Registry[db.DB]`: open on demand, bounded by `Config.MaxOpenTenants`
(256), idle stores closed after `Config.TenantIdleTTL` (5m), coldest evicted
first, never one that is mid-operation.

`Registry.Do(ctx, tenant, fn)` scopes a handle to a callback, on purpose — a
handle you can keep is one someone must remember to give back. `Manager.User`/
`Org` still return a `db.DB` because every call site keeps one (`Store.DB`, the
namespaced datastore resolver, the user service), but what they return holds no
handle: `db/tenant.go` is a VALUE that borrows the tenant's store for each
operation and gives it back. So a held `db.DB` costs four words, never a file
descriptor, and two consecutive operations on it may run on different open
handles — nothing needs otherwise, since anything atomic across statements is
already inside `RunInTransaction` (one borrow).

`Registry`'s `Materialize` (restore a tenant file from object storage on a local
miss) and `OnOpen`/`OnClose` (bracket a handle's life) are where
`hanzoai/replicate` attaches. Both are unset today: commerce tenant files are
still local-only, and turning them on needs an object-store config, not more
code here.

**PVC**: `commerce-data` (10Gi, do-block-storage) — deployment uses `Recreate` strategy (not RollingUpdate) because the PVC is ReadWriteOnce.

## At-Rest Encryption (SQLCipher via hanzoai/cek)

Per-tenant SQLite files hold money data (balances, transactions, usage), so they
can be encrypted at rest with `github.com/hanzoai/cek` on top of
`github.com/hanzoai/sqlite` v0.5.0 (SQLCipher AES-256) — the SAME derived-key
scheme cloud and IAM use, so there is one at-rest model platform-wide.
Code: `db/encryption.go`.

- **Master key from KMS, ONE source**: `COMMERCE_KMS_MASTER_KEY` (64 hex), read
  ONLY by `ResolveMasterKey()`. Materialised from `kms.hanzo.ai` into
  `commerce-secrets` by the existing `commerce-kms-sync` KMSSecret (path
  `/commerce`), same mechanism as `HUSD_TREASURY_KEY`. Never in git/code.
- **Derived, not wrapped**: a tenant's file key is `cek.DeriveKey(master, ns,
  "commerce")` — HKDF over the master, the tenant's `hanzoai/namespace` name and
  the subsystem. It is a pure function of the tenant's name, so there is NO DEK,
  no sidecar, no unwrap step, no rewrap and no per-file key material to lose. A
  file reopens after a restart with nothing persisted beside it. Losing the
  master loses the data, which is the property at-rest encryption is for.
- **Why DeriveKey and not `cek.Open`**: commerce opens a DUAL pool — a concurrent
  read pool and a serialized single-connection writer — against ONE file under
  ONE key. `cek.Open` hands back a single `*sql.DB`. The key is the shared part;
  the pool is commerce's.
- **Posture decided once**: unset key → unencrypted (dev/CI). Set + codec linked
  → encrypted. Set + cgo-but-no-libsqlcipher → **refuse** (`CodecLinked()`
  probe), never silent plaintext — commerce's dual pool cannot use the
  single-writer codec envelope a non-libsqlcipher build falls back to.
- **Layout**: an ORG's store is `<DataDir>/orgs/<slug>/commerce.db`, placed by
  `namespace.Path`, so the file's key and the file's path are two renderings of
  one name. This is the layout every Hanzo service shares.
- **Per-USER stores are UNENCRYPTED, and this is a real limit**:
  `namespace.Key` refuses `KindUser` — the shared layout has no place for a user
  — so a per-user store cannot be *named*, and therefore cannot be *keyed*, in
  this scheme. An encrypted open of a user tenant fails closed with an error
  naming the limitation (`TestEncryptedUserTenantIsRefused`); the unencrypted
  path is unaffected. Per-user files stay at `<UserDataDir>/<id>/data.db`. Giving
  `hanzoai/namespace` a user layout is the prerequisite for closing this.

**No migration.** Databases written under the previous DEK-sidecar scheme do not
open under this derivation, by design — there is no fallback, dual-read, version
probe or config flag, because a second way to key a file is how two of them end
up disagreeing. Wipe and recreate.

**Rollout:** build the image with libsqlcipher linked — add `libsqlite3` to the
build tags, `CGO_CFLAGS=-DSQLITE_HAS_CODEC -DSQLITE_USE_URI=1
-I/usr/include/sqlcipher`, `CGO_LDFLAGS=-lsqlcipher`, `apk add sqlcipher-dev`
(builder) + `sqlcipher-libs` (runtime), and a test stage with
`SQLITE_REQUIRE_CODEC=1` so a mis-link fails CI — then supply a 32-byte key.

**Where the key comes from:** an EMBEDDER passes `EmbedConfig.MasterKey` (cloud
hands over the KEK it already resolves, so one process has one key, not two); a
STANDALONE reads `COMMERCE_KMS_MASTER_KEY`. One source per deployment shape,
never both.

## Billing API (2026-02-23)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/billing/balance` | GET | Balance by user+currency (cents) |
| `/v1/billing/balance/all` | GET | All currency balances |
| `/v1/billing/usage` | POST | Record API usage (withdraw) |
| `/v1/billing/deposit` | POST | Create deposit transaction (per-subject money-in / settlement) |
| `/v1/billing/credit` | POST | THE ONE mint-gated, org-keyed credit grant (see below) |
| `/v1/billing/zap` | POST | Clear balance |

All require `permission.Admin` token (org live/test JWT). Cloud-api connects via `commerceEndpoint` + `commerceToken` env vars.

**Current org**: `hanzo` (ID: `gzh2BOBnV6gKZQ0CP`)

## One way to price a plan — the published catalog, applied by a DEPLOY

The plan catalog is `@hanzo/plans` (npm, public). `scripts/fetch-plans.sh` vendors
it into `api/billing/plans/*.json` at build time, `go:embed` bakes it in, and
`plan.Seed` reconciles the live `models/plan` rows to it on EVERY boot. So
changing a price is: edit the package → publish → re-vendor + bump
`PinnedPlansVersion` → bump commerce → bump cloud's `go.mod` → deploy. Every step
versioned, reviewed and revertible.

There is no second way. `cloud/scripts/seed-plans.sh` (a human curling production
with a SuperAdmin bearer, with the ladder retyped in bash) is DELETED — it was a
second statement of the ladder kept in step by hand, which is exactly how the
pricing page came to publish Pro at $49 while billing charged $20.

**The boot seed CAN change what a plan charges.** An older comment claimed the
opposite ("seed values EQUAL the embed, so it changes NO charge"); that stopped
being true when the seed gained the ability to correct its own prior output.

### Why the seed could not correct itself (and what fixed it)

`Managed` was set by BOTH `plan.Seed` and `api/plan`'s Create/Update, so one bit
carried two facts — "authoritative" and "who decided it" — and
`if existing.Managed { continue }` skipped every row the seed had ever written.
Publishing a new catalog would therefore CREATE the plans that were missing and
LEAVE every plan that had changed at its old price: half-new, half-stale.

`AdminEdited` is the discriminator that was missing. ONLY `api/plan` sets it.
`Seed` reconciles anything else to the catalog — the rule everyone already assumed
held: the package decides, an admin edit overrides. Proven end to end by
`api/billing/plans_converge_test.go`, which plants the real production catalog
(Managed rows, retired tiers, old prices) and boots it against the published one.

### Lifecycle: archive, never delete (the Shopify product model)

`plan.Status` is `active | draft | archived`, and **empty means active** — every
row written before the field existed has no value for it, and a zero-value meaning
"draft" would blank the catalog on deploy. `plan.Listed()` is the ONE predicate,
gating both halves of "on sale":

- the public catalog (`planAuthorityRows` skips unlisted rows), and
- the three purchase entrypoints — `CreateBillingSubscription`, the PATCH
  plan-change, and the card subscribe — which 404 an unlisted plan. Hiding a tier
  from the page is worth nothing if the API still sells it, and the card path
  refuses BEFORE the charge.

`resolveSubscriptionPlan` is deliberately NOT gated: it answers "which row is
this" and must keep answering for an archived plan, or a renewal on a retired tier
could not price itself. Retiring stops new sales; it never strands a subscriber.
Seed archives what the catalog stopped publishing; an admin-created plan is exempt
(it is not "missing from the catalog", it is theirs).

### The wire shape IS the model shape

A plan's display envelope (features/limits/bundles/includedIn) used to live only
as a packed JSON string in `Metadata`, with the packer private to `api/billing` —
somewhere the admin handler could not reach. So `PUT /v1/plans/entries/:slug`
carrying `features` set the price and SILENTLY DISCARDED the copy: a tier could be
repriced but never re-described. Those fields are now first-class on `plan.Plan`
(JSON-visible, `datastore:"-"`, still persisted packed under the same one key), so
whatever `GET /v1/billing/plans` emits, the admin CRUD accepts. `planLimits` is an
ALIAS of `plan.Limits`, not a second declaration.

### Gotchas this cost

- **A row from `GetAll` is a VALUE with no datastore binding.** `Update()` on it
  writes nowhere AND RETURNS NO ERROR. Re-load through the bound point query
  (`New(db)` + `Query().Filter("Slug=",…).Get()`) before writing.
- **`Save()` must pack into `Metadata`, not `Metadata_`.** The ORM stores a row as
  JSON of the struct and `Metadata_` is `json:"-"`, so a value written only there
  does not survive. Measured, not assumed.
- **`paidTier` counts a contact-sales plan as paid.** It stores `Price=0` because
  its price is null, not free; reading price alone made a negotiated tier
  self-serve, so a catalog row with a real allotment could be minted with no
  payment. That never fired before only because the tier with the large allotment
  also carried a large price — luck, not a gate.
- **A cgo build needs `-tags sqlite_math_functions`** (hanzoai/base asserts it at
  compile time). Without it v1.49.26–v1.49.37 each pushed a release tag and
  published NO image — twelve dead releases.

## One way to grant credit — POST /v1/billing/credit (2026-07-16)

`POST /v1/billing/credit` is the ONLY way credit enters an org ledger. It is
**mint-gated** (registered through `middleware.Mint`, which applies
`middleware.PlatformOnly`): only the internal
service token OR a global admin (`owner=="admin"`) reaches it — every
self-service / org-owner / no-auth caller is 403/401. A client-supplied mint
amount must never be self-service; that is the money-critical core (a user cannot
credit itself). Body: `{org, amountCents, reason, tag?, currency?, expiresAt?, idempotencyKey?}`,
org-keyed (the org POOL account, not a per-human subject), idempotent on
`idempotencyKey`.

**The mint gate declares itself.** A mint route is registered through
`middleware.Mint(api)` — `mint.Post("/deposit", Deposit)` — which BOTH applies
`PlatformOnly()` and records the route's method + full path in the package-level
registry. Registration is the single declaration, so the gate and the record
cannot drift. `middleware.MintRoutes() []MintRoute` exports that surface (after
mounting) for cross-service checks: cloud's `/v1/billing` bridge forwards with
the admin `COMMERCE_SERVICE_TOKEN`, which satisfies `MayMintMoney`, so its
`billingForwardable` allowlist must stay DISJOINT from `MintRoutes()` — an
assertion cloud can now make against this API instead of hand-copying the list.
Routes registered through `util/rest`'s deferred table (`api/affiliate`) carry
the same `PlatformOnly` chain explicitly but are not self-declaring.

Consolidated three prior handlers into this one: deleted self-service
`GrantStarterCredit` (old `/credit`), `PostMyWelcome` (`/me/welcome`), and
`GrantStarter` (`/grant-starter`). "Starter credit" is now a PARAMETERIZED call
(`tag=starter-credit`, `amountCents=500`, `expiry=+365d`) driven by the
`billing/credit` constants (reduced to data-only). The mint-surface guard
(`api/billing/mint_surface_test.go`) proves `/credit` is the sole gated credit
entry point.

**Backing = injected double-entry ledger (`billing/creditledger`).** Commerce runs
embedded in the cloud binary, where the AI spend-gate reads cloud's native
the ledger. Commerce must NOT import cloud, so the host injects a
`creditledger.CreditLedger` via `EmbedConfig.Ledger` (`embed.go` →
`creditledger.Set`). When set, `Credit` and `GetBalance` route to it — a credit
lands in the SAME per-org account the gate reads (one ledger, no split). When nil
(standalone), both fall back to commerce's own datastore (tagged `Deposit`). The
interface is compiler-enforced; cloud implements a ~40-line ledger adapter.

```go
type CreditLedger interface {
    Credit(ctx, CreditInput) (txID string, balanceCents int64, err error)
    Balance(ctx, org, currency string) (availableCents int64, err error)
}
type CreditInput struct { Org, Currency, Reason, Tag, IdempotencyKey string; AmountCents int64; ExpiresAt *time.Time }
```

## Cross-Compilation (2026-02-23)

Colima QEMU crashes Go's HTTP/2 and module loader on ARM Mac. Use zig for cross-compilation:

```bash
go mod vendor
CC="zig cc -target x86_64-linux-musl" CXX="zig c++ -target x86_64-linux-musl" \
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -mod=vendor -ldflags="-s -w -extldflags '-static'" -o commerce ./cmd/commerce/main.go
```

Push to GHCR (Docker Hub credentials not available locally):
```bash
docker build --platform linux/amd64 -t ghcr.io/hanzoai/commerce:hotfix .
docker push ghcr.io/hanzoai/commerce:hotfix
```

## App Engine Migration (2026-02-24)

All App Engine dependencies removed. Context handling modernized:
- `middleware.RequestContext()` (was `AppEngine()`) — stores Go request context in Gin
- `middleware.GetContext(c)` (was `GetAppEngine()`) — retrieves it; falls back to `c.Request.Context()`
- Gin context key changed from `"appengine"` → `"context"` everywhere
- Legacy aliases `AppEngine`/`GetAppEngine` kept as `var` for backward compat
- Legacy GAE Python utils moved to `.legacy/` (bulkloader, datastore-admin, salesforce-metadata)
- ORM: all models use `mixin.Model[T]` with generic CRUD, namespace support, and `orm.Register[T]()`

## OSS Contributor HUSD Payouts (on-chain, 2026-06-27)

25% of cloud revenue is payable to OSS contributors in HUSD (Hanzo USD
stablecoin) on the Hanzo EVM. The split + SBOM attribution + payout algorithm
live in `models/contributor/`; the executor is `cron/payout/contributor/`.

- **Executor**: `contributor.Payout(ctx, Config)` computes period revenue,
  splits 25% across components by real SBOM weight, and disburses each
  allocation by `PayoutMethod`: `credits` (CreditGrant), `stripe` (queued
  transfer), `crypto` (on-chain HUSD ERC-20 transfer).
- **Crypto path**: `executeCryptoPayout` → `util/blockchain.TransferToken`
  (geth-free seam) → `thirdparty/ethereum.transferToken` (luxfi/geth signing,
  legacy EIP-155 tx). The impl is registered by a blank import of
  `thirdparty/ethereum` in `cmd/commerce/main.go` — without it,
  `TransferToken` returns `ErrNoTokenTransfer` (crypto payouts skipped, never
  silently lost). go.work + `replace => ./thirdparty/ethereum` link it under
  the `GOWORK=off` Docker build.
- **Config** (KMS → env, `HUSDConfig.LoadFromEnv`): `HUSD_TOKEN_ADDRESS`,
  `HUSD_CHAIN_ID`, `HUSD_RPC_URL`, `HUSD_TOKEN_DECIMALS`, `HUSD_TREASURY_KEY`.
  Fails closed: no `HUSD_TOKEN_ADDRESS`/`HUSD_TREASURY_KEY` ⇒
  `ErrHUSDNotConfigured` (mainnet has no default token addr, so an unset deploy
  can't mis-pay). `HUSD_TREASURY_KEY` is KMS-only (org hanzo, path
  `/commerce-secrets`, key `HUSD_TREASURY_KEY` → KMSSecret `commerce-kms-sync`
  → `commerce-secrets` Secret). Non-secret config is plain env on the operator
  CR (`universe .../operator/crs/commerce-v1.yaml`).
- **Trigger**: `POST /v1/contributor/payouts/execute` (admin) runs the executor
  for the org namespace — dry-run by default, `?execute=true` to disburse.
  Mirrors `billing auto-recharge/run-all`; drive it from a curl CronJob:
  `curl -XPOST -H "Authorization: Bearer $COMMERCE_SERVICE_TOKEN" \`
  `http://commerce.hanzo.svc:8001/v1/contributor/payouts/execute?execute=true`.
  `GET /payouts/preview` shows the allocation without paying.
- **TESTNET-FIRST**: deployed config points at Hanzo testnet (chainId 36962,
  HUSD = "Lux Dollar" `0xc57b7eCE…4D66`, 18 decimals,
  `http://hanzod.hanzo-testnet.svc.cluster.local:9630/v1/bc/C/rpc`).
  Proven on testnet 2026-06-27: tx
  `0xe5cf03378e2d9dd121dfc5631fa112b2ea03717c9928167d54195ae785866978`
  (treasury → contributor, 25.50 HUSD, status 0x1, block 9).
  Live proof: `go test -tags onchain -run TestOnChainHUSDPayout_Testnet`.
- **MAINNET SWITCH (OFF; requires sign-off + funded mainnet treasury)**:
  `HUSD_TOKEN_ADDRESS=0xe9e32EF8aaECB68794Da3E1E9191b0a64CeC2c83`,
  `HUSD_CHAIN_ID=36963`,
  `HUSD_RPC_URL=http://hanzod.hanzo-mainnet.svc.cluster.local:9630/v1/bc/C/rpc`,
  and re-point `HUSD_TREASURY_KEY` to a funded mainnet treasury.
- **Gas note**: `thirdparty/ethereum` `GasPrice()` queries `eth_gasPrice`
  (respects the chain's 25 gwei `minBaseFee`); the old hardcoded `1` wei caused
  "transaction underpriced" on Hanzo EVM.

## Versioning (tag == binary)

`commerce.Version` is a `var` (commerce.go), default = the current release.
CI injects the immutable image tag at build time so `/healthz` `version` always
equals the deployed tag:

- `docker-deploy.yml` passes `VERSION=<git tag>` (build-arg) on `v*` tag pushes.
- `Dockerfile` strips the leading `v` and applies
  `-ldflags "-X github.com/hanzoai/commerce.Version=<ver>"`. It is the ONLY
  Dockerfile that builds the binary — `Dockerfile.sqfix`, `Dockerfile.prebuilt`
  and `Dockerfile.sqfix-prebuilt` are gone. All three packaged a binary compiled
  by hand on a laptop (zig cross-compile / hotfix builds), which is not how
  anything ships, and none was ever referenced by `hanzo.yml`. One had drifted
  to build tags that no longer compile, so the only thing they could still do
  was mislead.
- Branch builds leave `VERSION` empty → the in-source default holds.

Cut releases with a `v*` git tag (`git tag -aX vX.Y.Z && git push origin vX.Y.Z`)
so the build produces `ghcr.io/hanzoai/commerce:X.Y.Z` whose binary reports
`X.Y.Z`. Do NOT ship named tags (`:X.Y.Z-foo`) for prod — they re-introduce the
tag/binary drift (the live 1.42.31-testmode image reported binary 1.42.5 because
`Version` was a hardcoded const nobody bumped).

## commerce-site edge (no nginx)

`commerce.hanzo.ai` serves the marketing SPA via `ghcr.io/hanzoai/static`
(`Dockerfile.site`, `--spa`, :3000) — NEVER nginx. The host's routing lives in
the hanzoai/ingress file provider (`universe infra/k8s/ingress/routes.yaml`):
a high-priority `commerce-hanzo-ai-api` router sends `/v1`, `/api`, `/healthz`
to the commerce API (`commerce.hanzo.svc:8001`); the low-priority
`commerce-hanzo-ai` router catches the rest → the SPA. The operator
`commerce-site` Service CR has `ingress.enabled: false` (a plain Ingress can't
express Traefik priority). The from-source `Dockerfile.site` Next.js build is
currently broken (unbuilt workspace dists, a phantom `@kapaai/react-sdk` import,
a `@/providers` alias); 0.2.0 re-serves the proven 0.0.1 assets — fix the app
build before bumping content.

## SBOM-driven OSS-developer payout (2026-06-25)

Every deploy emits an SBOM; Hanzo pays upstream OSS developers **up to 25%** of
all Hanzo cloud spend, attributed by the dependencies actually deployed. One
pipeline, decomplected into orthogonal pieces.

### The pipeline (one way, all images)

```
arcd build  --syft-->  CycloneDX SBOM  --normalize-->  POST /v1/billing/sbom
  (deploy)             (per image digest)               (ZAP 0x20 OR HTTP)
                                                              |
org usage  -->  RecordUsage  --go-->  engine.AccrueOSSPayout  v
(charge)        (existing)            (mirrors TrackRevenueShare)
                                          |
                  ossattr.Attribute(spend, SBOM pkgs, policy)   <= 25% pool
                  ossfunding.Resolve(purl) -> maintainer target
                                          |
                                          v
                  OSSAccrual ledger lines (per package, idempotent)
                                          |
                  GET /v1/billing/oss-payout/summary  (disbursement view)
```

### Components

| Concern | Where | Notes |
|--------|-------|-------|
| **Attribution math** (the heart) | `ossattr/` (leaf, stdlib-only) | Pure `Attribute(spendCents, []Package, Policy) -> Result`. 25% cap is `ossattr.MaxPoolFraction` (code, not config). Pro-rata by weight; largest-remainder apportionment conserves the pool to the cent. Deterministic + sorted. |
| **Policy** | `config/oss-payout.json` + `config/oss_payout.go` | `poolFraction` (clamped <=0.25), `directWeight` (1.0), `transitiveWeight` (0.25). Embedded JSON, `sync.Once` (mirrors referral-program.json). |
| **Maintainer resolution** | `ossfunding/` (leaf) | `FromPURL` (pure, offline): github.com/<owner> PURLs -> `github_sponsors:<owner>`; golang.org/x/* -> `golang`; npm/pypi w/o VCS -> Unresolved -> **held**. Network FUNDING.yml/registry enrichment behind `Resolver` (out of hot path). |
| **SBOM storage** | `models/sbomrecord/` | Per image digest; `Ingest()` is the ONE upsert (idempotent on digest), shared by HTTP + ZAP. Components are a noindex JSON blob. Global "system" namespace. |
| **Accrual ledger** | `models/ossaccrual/` | Append-only per-package line; idempotency key = `ossattr.AccrualID(org, txn, purl)`. Status: pending\|held\|queued\|settled. "system" namespace (Hanzo->OSS liability, aggregated across orgs). |
| **Accrual engine** | `billing/engine/osspayout.go` | `AccrueOSSPayout(...)` — fire-and-forget hook in `RecordUsage`, structural twin of `TrackRevenueShare`. Unions SBOM components, runs `ossattr.Attribute`, resolves funding, writes lines. |
| **HTTP surface** | `api/billing/osspayout.go` | `POST /sbom`, `GET /sbom`, `GET /oss-accruals`, `GET /oss-payout/summary` (admin-token). |
| **ZAP-native path** | `infra/zap_osspayout.go` | `OpSBOMIngest = 0x20` on the existing ZAP node; `RegisterSBOMIngest(store)` wired in commerce.go to the SAME `sbomrecord.Ingest`. |
| **SBOM emit (build side)** | `arc/cmd/arcd/sbom.go` | `dockerBuilder.emitSBOM` after `docker push`: `syft <digest> -o cyclonedx-json` -> `normalizeSBOM` (pure; direct/transitive from the dep graph) -> POST. Env: `SBOM_INGEST_URL/TOKEN/ORG`. Best-effort (never fails the build). |

### The 25% math

`pool = floor(spend * min(poolFraction, 0.25))`. Each package weight =
`base(scope) * criticality` (direct base 1.0, transitive 0.25, criticality
default 1.0). `share_i = pool * weight_i / sum(weight_j)`, apportioned to integer
cents so `sum(shares) == pool` exactly. Same PURL across SBOMs is de-duped
(highest weight wins) so each package is paid once. No packages / zero weight ->
whole pool **held**, never invented or lost.

### Proven (tests, all green)

- `ossattr`: cap enforced (incl. over-cap clamp), cent conservation over 5000
  spends, direct=4x transitive, criticality, dedup, held-pool, determinism, big
  spend exactness.
- `ossfunding`: PURL->target resolution incl. held cases.
- `billing/engine` (real SQLite): deploy SBOM -> $100 spend -> **2500c (25%)**
  across 4 pkgs; gin direct=1000c->`github_sponsors:gin-gonic`(pending),
  react transitive=250c->**held**; idempotent (2 fires -> 1 line); no-SBOM no-op.
- `api/billing` (HTTP, real router): ingest -> summary rollup; cap + held +
  resolution proven over the wire.
- `arc/cmd/arcd`: `normalizeSBOM` CycloneDX->ingest with direct/transitive
  graph tagging + PURL-less drop.

### Money-out (designed, creds-gated — NO fake payouts)

Accrual is real + provable now. Disbursement is a **separate** job that reads
`GET /oss-payout/summary` (status=pending, resolved targets), batches by funding
kind, pays, and flips lines pending->queued->settled (referencing a `Payout`).
Rails are integration/creds-gated:
- **GitHub Sponsors** — no programmatic payout API; needs the Sponsors GraphQL +
  an org Sponsors account funding source. Per-maintainer sponsorship.
- **Open Collective** — API for expenses/payouts; needs an OC account + API key.
- **Stripe Connect** — `Transfer`/`Payout` to connected accounts; needs the
  maintainer to onboard a connected account (KYC). Best general rail.
All require: a funded Hanzo source account, per-rail API credentials in **KMS**
(never plaintext), maintainer onboarding/KYC, tax handling (1099/W-8). The held
pool waits until a target is resolved + onboarded.

## Three money buckets — credits vs prepaid vs card (billing/bucket)

The ledger was CONFLATED: `starter-credit`/`included-credit:` (non-cash grants)
and `topup`/deposits (real money) were all `transaction.Deposit` rolled into ONE
`balance`. They are now split HONESTLY from the real tags — no fabricated numbers.

- **`billing/bucket`** (pure, no I/O, unit-tested) is the ONE classifier + spend
  projection:
  - `DepositKind(tags)` → `Credit` (tag `starter-credit`, prefix `included-credit:`,
    `credit:`, `grant:`) vs `Prepaid` (everything else — topup/husd/bare deposit =
    real money; UNKNOWN→Prepaid is fail-closed: never mint spendable-on-GPU value
    from a mystery deposit).
  - `IsGPUWithdrawal(tags)` → gpu (`gpu`/`gpu-*`/`gpu:*`).
  - `Compute(transs, id, now) → Split{CreditsGranted, CreditsRemaining,
    PrepaidBalance, PrepaidAvailable, Balance, Holds, Available}`.
- **Spend policy** (documented, enforced): non-GPU usage draws CREDITS FIRST then
  prepaid; GPU usage draws ONLY prepaid. The projection reconciles to the cent:
  `CreditsRemaining + PrepaidBalance == Balance` (the same balance the gateway
  gate debits — bucket.Compute is a faithful superset of util.TallyTransactions).
- **Read**: `GET /v1/billing/balance` + `/me/balance` now ALSO return
  `creditsGranted`, `creditsRemaining`, `prepaidBalance`, `prepaidAvailable`, and
  `card{onFile,brand,last4,isDefault}` (additive — the pre-split
  `{balance,holds,available}` is unchanged, so the cloud proxy + console BFF
  forward the split verbatim; console shows credits vs prepaid distinctly).
  `util.GetRawByCurrency` fetches the raw (un-expiry-filtered) ledger the split needs.
- **GPU rule (the bucket projection, `billing/bucket`)**: a `gpu`-tagged Withdraw
  draws PREPAID only — `IsGPUWithdrawal` classifies it and `Compute` never lets it
  reach a credit grant. It is a property of the LEDGER, so it holds for every
  writer rather than for one endpoint. There is no bespoke GPU charge door here:
  a GPU is a metered resource like any other and is billed through the general
  metered path (a machine is launched via cloud `/v1/machines`, which fronts the
  compute provider's resell endpoint where the balance gate and the per-hour meter
  live). A GPU charge NEVER calls `BurnCredits`.
- **CreditGrant** (`models/creditgrant`, `BurnCredits`) is a SEPARATE grant ledger
  used only for subscription/invoice collection (cycle/invoices/subscriptions) —
  NOT the pay-as-you-go wallet the balance endpoint reads. Left as-is.

## Chain-Backed Credit Ledger — HUSD-on-Hanzo-EVM (steps 1-6 BUILT + testnet-proven)

Kills the recurring C1 money-mint vuln class by construction: a credit exists
ONLY as HUSD (Hanzo USD, ERC-20 on the Hanzo EVM) minted by the KMS treasury key
— which commerce does NOT hold. Commerce can only REQUEST a mint; it becomes a
read-only indexer of on-chain balances. No commerce code path can create money.

**Foundation landed (branch `feat/chain-backed-credit-ledger` / `chain-ledger-wip`,
7-step plan — steps 1-3 done, 4-7 staged):**

1. **`treasury.DeriveAccount(masterSeed, orgID)`** (`treasury/derive.go`) — per-org
   on-chain address, BIP-32-style HMAC-SHA512 → secp256k1 (`luxfi/crypto`, no
   geth). Deterministic across restarts, collision-safe for string org ids,
   golden-vector pinned. Private key (settlement-signing, step 5) in memory only.
2. **`treasury.Mint(ctx, MintRequest)`** (`treasury/treasury.go`) — the ONE mint
   path: treasury-signed on-chain HUSD transfer to the org address. TWO locks:
   `mintauth.Require(ctx)` (gated HTTP caller needs proven authority; crons
   ungated) + KMS key custody (`husd.Config.Configured()` else `ErrNotConfigured`,
   no DB fallback). Idempotent by key (deterministic `IssuanceID`; replay → stored
   receipt, no new tx; concurrent same-key → exactly ONE on-chain mint). Bucket
   (credit|prepaid) recorded off-chain → ledger tag (`credit:husd`|`husd`).
   Production store: `models/husdissuance` (kind **281**) + `treasury/datastorestore`
   (deterministic-id ON CONFLICT, mirrors `models/idempotencykey`).
3. **`billing/husdindex`** — chain is source of truth. `client.go` = CGO-free
   JSON-RPC read client (BlockNumber/BalanceOf/TransfersTo). `index.go` = `Sync`
   projector: scan Transfer events → project idempotent bucket-tagged credits
   (dedup on `txHash:logIndex`), reconcile indexed == `balanceOf` to the cent.

- **`util/husd`** = the ONE HUSD config + cent↔wei source (contributor payout
  aliases it now — DRY). `mintauth.Require` = the shared "gated & unauthorized ⇒
  refuse" policy (sink `Enforce` + mint service both call it).
- **Decomplected for CGO-free proofs**: core logic (derive/mint/index) has no
  sqlite/geth dependency, so unit tests + the live proof run `CGO_ENABLED=0`
  locally (the `luxfi/accel` cgo link + `mattn/go-sqlite3` both fail on a laptop;
  datastore adapters are CI-tested). geth `core/types`/`rlp` are avoided in the
  live signer too — their transitive `luxfi/pq@v1.0.3` pull trips the upstream
  force-moved-tag go.sum mismatch; the proof signs EIP-155 with `luxfi/crypto` +
  hand-rolled RLP (production uses the geth `thirdparty/ethereum` signer via the
  same `util/blockchain.TransferToken` seam).

**PROVEN on testnet (chainId 36962), end-to-end, independently verified via cast:**
- Provisioned a fresh HUSD test token `0xe7f1725e7734ce288f8367e1bb143e90bb3f0512`
  (the ephemeral testnet had reset — no token, unfunded treasury; funded treasury
  `0xe6da…a51a` from the genesis hardhat account `0xf39F…2266`, minted 1M HUSD to
  treasury). Node serves the new `/v1/bc/C/rpc` surface (post `/ext→/v1` cutover).
- Live mint tx `0xd01bc1c1733e83c93a5143552af1ba7e3ac045b2433c2fecff07a29685cf976a`
  (status 0x1, block 25): `treasury.Mint($12.34, credit)` → treasury → derived
  org addr `0x3560…9950`. Org `balanceOf` == 1234c; treasury balance dropped by
  EXACTLY 12.34 (value from real supply, not fabricated); indexer `Sync` projected
  a `credit:husd` 1234c credit == on-chain; replay of the idem key sent NO second
  tx. Run: `HUSD_RPC_URL=… HUSD_TREASURY_KEY=… HUSD_TOKEN_ADDRESS=… CGO_ENABLED=0
  go test -tags onchain -run TestOnChain_ChainBackedLedger ./treasury`.

**Headline: on testnet, minting is treasury-only (commerce holds no key, cannot
mint) and idempotent, and the indexer reflects on-chain balance — proven live.**

### Steps 4-6 (BUILT + testnet-proven, branch `chain-ledger-wip`)

4. **Mint-path rewire** (`billing/husdledger` + `api/billing/husd_mint.go`).
   `husdledger.Service` is the ONE mint entrypoint (`MintCredit`): treasury
   on-chain mint → bounded synchronous `ProjectTx` so `GET /v1/billing/*/balance`
   reflects it on return; a background/CronJob `SyncOnce` backfills. Adapters:
   `ledgerStore` (projects a bucket-tagged `transaction.Deposit` — the SAME row
   the balance read tallies — keyed to the mint's `Subject` in its `Test`
   partition, idempotent on `txHash:logIndex`), `addressBook` (derive every org
   address from the org list + seed), `cursorStore` (`models/husdcursor` kind
   **282**), and `datastorestore.ByTxHash` (the issuance store IS the
   `IssuanceLookup`, system-ns). `MintRequest`/`Issuance`/`Credit` gained
   `Subject` (per-org address, per-subject ledger) + `Test`. Handlers
   `Deposit`/`GrantStarterCredit`/`PostMyWelcome`/`GrantStarter`/`topup` branch to
   the chain mint when enabled (else the DB path, unchanged). Wired at Bootstrap
   from `husd.Config` + `HUSD_ORG_DERIVATION_SEED`. The **seed is the ledger's
   sole intent signal** (`husdledger.ValidateConfig`, PR #69): ENABLED only with
   a seed AND token+key; token+key WITHOUT a seed is the INERT state
   (`Enabled()==false`, DB credit path) — NOT a boot error — because the shared
   `util/husd` token+key is ALSO the OSS contributor-payout config, which runs
   with no ledger seed. Boot refuses ONLY on an incoherent LEDGER config (seed
   set but token/key missing). This decouple is why v1.46.39 boots inert in prod
   (the CR carries HUSD token+key for OSS payouts, no seed) instead of
   crash-looping; the mainnet cutover enables the ledger by provisioning the seed.
   **Proven**: live `TestOnChain_Step4_ProjectTx` (mint `0x3324a75a…` → ProjectTx
   from the REAL receipt → one credit subject/bucket/test) + CI `ledgerStore`
   sqlite test (ProjectTx → real Deposit → `bucket` balance, idempotent).
5. **Settlement** (`husdledger/settlement.go`, `husd.SettlementDrift`, kind **283**).
   `drift = balanceOf(orgAddr) − max(0, ledgerSpendable)`; sweep org→treasury,
   signed by the ORG's derived key, when `drift ≥ threshold`. Self-correcting +
   idempotent. `POST /v1/billing/husd/settle` (platform-only). **Proven**: live
   `TestOnChain_Step5_Settlement` (mint $50 → gas-fund org → org-key sweep
   `0xf323bf25…` → org drops EXACTLY $20 == spendable, treasury +$20) + CI
   `SettleOrg` sqlite test (real ledger $50−$20 → sweep $20, reconcile, idempotent).
6. **Migration + reconcile** (`husdledger/migration.go`). Snapshot each subject's
   `bucket.Split` → treasury-mint the equivalent per bucket → neutralize legacy
   deposits with two idempotent offset withdraws (non-GPU cancels credit
   credits-first; GPU-tagged cancels prepaid prepaid-only) so the split is
   preserved → assert chain `balanceOf` == pre-migration DB balance == post DB,
   exact to the cent, zero settlement drift. `POST /v1/billing/husd/migrate`
   (dry-run default, platform-only). **Proven**: CI `MigrateOrg` sqlite+fakeChain
   test (alice 2500c/2000p + bob 1000p → reconcile 5500==5500==5500, buckets
   preserved, idempotent).

### Step 7 — DB-mint deletion (the mainnet cutover GATE, NOT done this session)

The named money-in paths (deposit/welcome/topup) route through the chain when
enabled; the DB write stays as belt-and-suspenders (bypassed on the chain path,
still guarded by #65's `mintauth` sink) until prod migration is 100%. Before the
deletion, these REMAINING credit-mint surfaces MUST also route through
`chainMintCredit` (else they DB-mint credit outside the chain when enabled):
`billing/allotment` (monthly included-credit), `billing/trial`,
`billing/credit.GrantIfEligibleNow` (shared auto-grant helper),
`api/billing/refund.go`, `api/billing/topup_token.go`, `api/billing/webhooks.go`
(settled-payment deposits), `api/transaction/create.go` (raw `/v1/transaction`).
(`api/checkout/util/capture.go` credits an ORDER wallet, not the gateway-spendable
iam-user wallet — out of scope.) THEN delete the DB deposit writes; the
`mintauth` sink stays as the final backstop.

**Mainnet cutover sign-off gate** (separate; do NOT touch mainnet HUSD):
(a) route the remaining credit-mint surfaces above; (b) provision the mainnet
HUSD token + a FUNDED mainnet treasury (`HUSD_TOKEN_ADDRESS`, `HUSD_CHAIN_ID=36963`,
`HUSD_RPC_URL=…hanzo-mainnet…`, `HUSD_TREASURY_KEY` + `HUSD_ORG_DERIVATION_SEED`
in KMS) with gas float per org address (settlement needs it); (c) run
`/husd/migrate?execute=true` per org, assert reconcile; (d) enable the chain path;
(e) run `/husd/sync` + `/husd/settle` on a CronJob; (f) after migration is 100%,
land step 7's deletion. Testnet-first is complete; mainnet is the sign-off.

Config: `HUSD_TOKEN_ADDRESS`, `HUSD_CHAIN_ID`, `HUSD_RPC_URL`, `HUSD_TOKEN_DECIMALS`,
`HUSD_TREASURY_KEY`, `HUSD_ORG_DERIVATION_SEED` (all KMS), `HUSD_INDEX_CONFIRMATIONS`
(default 1), `HUSD_SETTLE_THRESHOLD_CENTS` (default 1). Testnet proof env: token
`0xe7f1725e…0512`, chainId 36962, RPC `…/v1/bc/C/rpc`, treasury `0xe6dad4…a51a`.

## Crypto deposit watcher — the half of the crypto rail that credits (2026-08)

`POST /v1/billing/crypto/deposit` mints a per-payer MPC custody address and
records a `CryptoPaymentIntent` as Pending. Until now NOTHING advanced it: no
component observed `DepositAddress`, `MarkConfirming`/`MarkSucceeded` had zero
production callers, and money sent to a minted address was received and never
credited. That is why `cryptoDepositsCanBeCredited` (api/billing/crypto_deposit.go)
is `false` and the handler 503s before any keygen.

The watcher is now built. **The gate is still `false`** — see "before flipping".

**Two packages, decomplected exactly like husdindex/husdledger:**

- **`billing/depositwatch`** — pure policy over small interfaces (no datastore,
  no HTTP): the scan window, the confirmation rule, the amount arithmetic, the
  address→intent match. Unit-tested against fakes with no chain and no DB.
- **`billing/depositledger`** — the I/O half: per-org intent store, the
  idempotent ledger write, the persisted cursor (`models/depositcursor`), and
  the schedule. Integration-tested against a real datastore (`util/test/ae`).

Chain reads reuse **`husdindex.Client`** — this repo's ONE ERC-20 JSON-RPC read
client, parameterized by `(rpcURL, tokenAddr)` and named for its first caller,
not for a restriction. Extended with `Decimals()` / `Symbol()` (selectors pinned
against `luxcrypto.Keccak256` in `client_test.go`).

### Exactly-once = a KEY, not coordination

A credit's storage id is `sha256("crypto-deposit\0" + chain:txHash:logIndex)`
(`depositledger.creditKey`). Both backends upsert on `(id, kind, namespace)`
(db/sqlite.go, db/postgres.go), and a balance is a SUM over rows, so every
attempt to credit the same transfer lands on the same row: re-scan, crash retry,
and N replicas ticking simultaneously all yield one credit. **No leader election,
no lease, no lock** — adding replicas cannot change the answer. The chain is part
of the key because a pre-EIP-155 tx can be replayed across EVM chains with an
identical hash.

Ordering is load-bearing: **ledger row FIRST, intent second.** The reverse would
let a succeeded intent survive a failed ledger write, and a later pass would read
it as done — money received, never credited.

### Scan ahead, commit behind

Scan `[cursor+1 … head]` every pass (so a customer sees "confirming" within a
block); commit only `head − requiredConfirmations`. The last `required` blocks
are therefore re-read from the canonical chain every pass, which is what makes
reorgs harmless: a sighting that vanishes is un-sighted back to Pending
(`CryptoPaymentIntent.ClearSighting`), a sighting that MOVES block is re-read at
its new height. Un-sighting only fires for blocks actually inside the scanned
range — silence about a block we never read is not evidence.

Cold start (empty cursor) begins at `head − required`, so no block is ever
committed unscanned and no public chain is scanned from genesis.

Confirmation depth comes from `cryptopaymentintent.RequiredConfirmationsForChain`
— the ONE policy, now naming every supported chain (eth 12, rollups+polygon 20,
bsc 15, avalanche/lux/zoo 12, solana 32).

### Decimals are READ, contracts are VERIFIED, tokens are PEGGED

Getting decimals wrong by 12 credits 10^12 too much, so decimals are never
configured: `verify()` reads `symbol()` and `decimals()` off the contract once
per asset, refuses the asset if the symbol disagrees with the configured token,
and refuses `< 2` or `> 36` decimals. A transient RPC failure is not cached.

Config (KMS-injected, NO defaults — mirrors `util/husd`'s "token address has no
default" rule):

```
CRYPTO_DEPOSIT_RPC_<CHAIN>            e.g. CRYPTO_DEPOSIT_RPC_BASE
CRYPTO_DEPOSIT_TOKEN_<CHAIN>_<TOKEN>  e.g. CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x8335…
```

Unset ⇒ watches nothing (inert). Incoherent (token with no endpoint, unpegged
token, malformed address) ⇒ **Bootstrap refuses to start**: an operator who
configured a deposit rail must not get a process watching less than they asked
for.

`depositwatch.pegCents` IS the creditable set (`usdc`, `usdt` @ 100c). There is
no price oracle in commerce and inventing one to value a customer's money would
be guessing, so **only dollar-pegged ERC-20s are creditable**. Native coins
(ETH/MATIC/AVAX/BNB) are doubly out: a native transfer emits no log so
`eth_getLogs` cannot see it at all, and it cannot be priced. Known accepted risk:
a depegged stablecoin is credited above market.

### It is SCHEDULED, in process

`depositledger.Service.Start()` from `Bootstrap`, stopped in `Shutdown` before
the DB closes. Interval is a constant (30s), not config. This is the FIRST
in-process scheduler in this repo, and deliberately so: every other periodic job
here is an endpoint waiting for a CronJob defined in another repository, which is
exactly why `/v1/billing/husd/sync` has never once run. Safe on every replica
because a pass is idempotent (above).

### Scaling note

`Watched()` returns every minted address, in every org, with NO status filter —
money sent to an expired or already-settled intent is still the customer's money.
Address count therefore grows with total deposits ever taken; `eth_getLogs` calls
are chunked at 100 addresses / 2000 blocks. If an expiry sweeper is ever added,
"stop handing the address out" and "stop watching it" are separate decisions and
only the first is safe.

### Before flipping `cryptoDepositsCanBeCredited`

1. Provision `CRYPTO_DEPOSIT_RPC_*` + `CRYPTO_DEPOSIT_TOKEN_*` in KMS and confirm
   the boot log says `crypto deposit watcher ENABLED (…)`.
2. ~~Restrict the offered token set.~~ **CLOSED in v1.50.19.** `GetCryptoOptions`
   used to serve the MPC processor's list (btc, eth, matic, avax, lux, zoo, arb,
   op, base…) — which answers "can I DERIVE an address here?", never "can this be
   credited?" — so it walked buyers toward assets the watcher credits none of.
   It now projects the watcher's configured assets through `offeredFrom`, so the
   menu cannot name an asset nothing is watching, and an unconfigured watcher
   offers nothing rather than everything. Nothing to remember at flip time: the
   list narrows and widens with the config by construction.

   ⚠ `offeredFrom` is a PURE function and tested as one, deliberately. The first
   test drove the handler end-to-end and passed against a mutant serving a
   hardcoded list — the custody-signer probe fails in a test environment, so the
   handler answered 503 and the assertion never ran. Mutation-test anything that
   guards money; a green endpoint test can be measuring nothing at all.
3. ~~`GenerateAddress` DISCARDS the keygen `wallet_id`.~~ **CLOSED in v1.50.20.**
   It returned a bare string, so the id was parsed off the keygen response and
   dropped — commerce minted custody addresses it held no handle to, and a
   credited deposit could not be SWEPT without reconciling against the node's own
   records. `GenerateAddress` returns a `processor.Wallet{Address, ID}` and
   `CryptoPaymentIntent.WalletID` records it at mint time, the only moment the
   signer offers it. The existing method was WIDENED rather than joined by a
   second one: two ways to mint an address is how the halves drift apart again.

4. **Still open — the creditable set is dollar-pegged ERC-20s only.** Native
   coins (ETH/LUX/ZOO/MATIC…) emit NO log on a bare transfer, so `eth_getLogs`
   cannot see them, and commerce has no oracle to value one. Same two blockers
   for XRP, and additionally XRPL's 10 XRP base reserve is non-refundable, so it
   wants ONE pooled r-address with per-intent destination tags rather than an
   address per payer. ⚠ "needs no new cryptography" describes address derivation
   and says nothing about whether a deposit can be credited.

5. **Still open — SOL and TON have no signable key.** `frost.KeygenTaproot` is
   BIP-340 **secp256k1**; its 32-byte output passes mpcd's `len(EDDSAPubKey)==32`
   gate and base58-encodes into a real-looking Solana address the ring can never
   sign for. The empty `eddsa_pub_key` is the system failing closed — leave it
   until a genuine RFC 8032 ciphersuite exists in `luxfi/threshold`.

## zip / ZAP native — what is real, and why billing is NOT typed yet

zip is the ZAP-native framework: a typed op (`zip.Get/Post[In,Out]`) is ONE value
with THREE projections — REST, OpenAPI, MCP. Only typed ops populate `app.ops`;
untyped `app.Get(path, func(c *zip.Ctx) error)` projects to REST and nothing else.
Commerce has exactly one zip route (`/_/commerce/healthz`) and it is untyped.

**zip is pinned at v1.8.3** (was v1.2.0). The climb v1.2.0 → v1.8.3 has exactly
ONE breaking change: **v1.6.0 removed `App.Mount`**. It was a one-liner
(`a.All(prefix+"/*", AdaptNetHTTP(h))`), so `mount.go` now calls that directly —
same route set. v1.3.0 swapped `gofiber/fiber/v3` → `zap-proto/fiber/v3`
(transparent; commerce never imports fiber). v1.8.3 alone adds `App.Authorize`
and `App.Prepare` — v1.8.2 and earlier have NEITHER.

### Why the /v1/billing surface is still gin (four independent blockers)

1. **Typed ops bind the BODY ONLY.** `registerTyped` passes `c.Body()` and
   nothing else; for GET/HEAD it does not even read that, so `In` is the zero
   value. There is no path-param, query-param or header binding at ANY version
   through v1.8.3. `GET /billing/invoices/:id`, `?customerId=`, and Deposit's
   `X-Idempotency-Key` (which makes the mint exactly-once) are all unexpressible
   without a wire change. Changing the money wire to satisfy a framework is
   backwards.
2. **The MCP projection bypasses route middleware.** `mcpCall` invokes the op
   core directly, so `middleware.PlatformOnly` never runs. On v1.8.2 (what cloud
   pins) there is NO authorizer, so this is ungateable: a typed `billing.deposit`
   would be an unauthenticated mint tool. `App.Authorize` (v1.8.3) closes it and
   gates REST + MCP with one decision — but it is a single app-level SETTER, so
   on cloud's shared App the last subsystem to call it silently wins.
3. **The mint gate is keyed on the CONCRETE TYPE `*gin.Context`.**
   `Organization.Namespaced` type-switches: a `*gin.Context` gets
   `mintauth.WithGate`; anything else is treated as an internal cron and is
   **NOT gated** (`mintauth.Require` returns nil when `!Gated`). A typed op holds
   a plain `context.Context`, so moving a ledger handler off gin silently
   disarms the mint invariant — no compile error, no test failure. Pinned by
   `models/organization/namespaced_ctx_test.go:TestNamespaced_InternalCallerUngated`.
4. **The tenant key is only in the gin context.** `GetOrganization(c)` is
   `c.MustGet("organization")`. A typed op has no org ⇒ no tenant scoping.

Blockers 3 and 4 are the real work, and it is NOT a billing change: identity +
datastore ctx must flow through `context.Context` (set once at the boundary)
before ANY handler can be typed. That is the actual phase 1.

`mount_test.go:TestMount_NoUngatedTypedOpSurface` is the tripwire: it asserts
`/mcp` 404s after `Mount`+`Prepare` (commerce registers zero typed ops), and
fails the moment one appears. Verified to fire.

### /v1/billing is DEAD through the cloud mount

`mount.go` calls `api.Route(apiGroup)` on the `/v1` gin group, registering ~140
`/v1/billing/*` routes — but the zip mount only exposes `/v1/commerce/*` and
`/_/commerce/*`. Measured through `Mount`: `/v1/billing/balance` and
`/v1/billing/deposit` return **404 with fiber's body** (`{"status":404,...}`, not
gin's NoRoute `{"error":"not found"}`) — the request never enters the gin engine.
The comment at `mount.go` claiming it wires "the full Commerce API surface
(/v1/billing, …)" is false for every prefix except `/v1/commerce`. In cloud,
`/v1/billing/*` is served by cloud's own `clients/billing` + `clients/account`
wildcard, NOT by commerce. Do not "fix" the prefix casually: exposing 140 routes
into cloud's tree collides with those, and a collision is a BOOT PANIC.

**ONE ADDRESS, ONE DECLARATION.** This paragraph used to say byte-identical
patterns "MERGE silently, first wins". Measured 2026-08-06, they do not — zip
refuses the whole program and names every conflicting pair:

```
zip: this program does not compose, so it has no projection:
zip: GET /v1/commerce/collection: declared by "/collection" at rest/rest.go:133
  (via root → /v1/commerce → /collection) and by "/collection" at rest/rest.go:133
```

Where it lands depends on the zip in the graph, and **a check that only
REGISTERS measures nothing**: this repo pins v1.24.2, which refuses inside
`rest.Route()`; cloud resolves v1.25.1, which accepts the second call and
refuses at COMPOSITION (`GetRoutes()`, serve). Ask for the composition.

The live consequence: `api/resources` is also called by `api.Route` (api.go:125),
so a kind added to that leaf while api.go still registers it **panics this
binary at boot** — and the change that caused it may have been made in cloud.
Moving a kind onto the leaf is a move, never an add.

### schema/commerce.zap — dialect and the real compiler

`schema/commerce.zap` was proto-ish fiction (`//` comments, `service`/`message`,
`= 1` field numbers) that no tool could read. It now COMPILES, and is the only
`.zap` in the fleet that does — `ai`, `kms`, `vfs`, `iam`, `o11y` and
`api/billing/billing.zap` all fail with `expected 'package' declaration`.

- The compiler is **`zapgen`** (`hanzo/zap/go/cmd/zapgen`, Go-only:
  `-out/-single/-type-suffix`), NOT `zapc`. `zapc` is the Cap'n-Proto-lineage
  RUST codegen PLUGIN that reads a `code_generator_request` on stdin; it has no
  `generate --lang go|ts|py|rust` verb. That command, documented in every `.zap`
  header in the fleet, does not exist.
- Dialect: `#` comments; `package` REQUIRED first; only `struct` / `interface` /
  `type` (**no enum**); lowercase primitives (`text i64 i32 bool bytes list<T>`);
  methods are `name(param: Type) returns (out: Type)`. The braceless
  whitespace form is sugar — `Desugar` adds the braces and auto-assigns each
  field's `@byteOffset`.
- Verify: `GOWORK=off go build ./cmd/zapgen && zapgen -out /tmp/gen schema/commerce.zap`
  emits a `BillingClient` + `BillingHandler` + `DispatchBilling` ordinal router
  that builds clean.
- The generated client is **not committed**: nothing consumes it, and the
  hand-rolled `ZapDispatch` (api/billing/zap.go) already serves that surface.
  Committing it would create a second, competing billing client. `interface
  Billing` mirrors exactly the five methods `ZapDispatch` really routes —
  method ordinals are wire identity, so APPEND only.

## Gotchas

- New ORM kinds MUST be registered in `util/hashid/kind.go` (monotonic, never
  reorder) or `Create()` panics "Unknown kind" — `sbom-record`=262, `oss-accrual`=263,
  B2B `company`=264/`employee`=265/`quote`=266/`quote-message`=267/`approval`=268,
  `gift-card`=269/`gift-card-redemption`=270, `exchange`=271, `idempotency-key`=272,
  `husd-issuance`=281 (chain-backed credit ledger).

## Medusa parity (native Go /v1 — 1.42.41)

The `/v1` model bundle (`api/api/api.go`, mounted at `/v1/*` by `cmd/commerced`
+ `mount.go`; the `/v1/commerce/*` prefix is the SEPARATE checkout/tenant surface)
covers Medusa v2's admin domains natively — no Medusa/Node fork. Reference:
`~/work/medusa/medusa/packages/medusa/src/api/admin/*` + `packages/modules/*`.

### Newly wired (models existed, routes were orphaned → 404 in prod)
`api/api/api.go` now calls these 5 previously-unwired sub-routers:
- `fulfillment` — fulfillmentset, servicezone, geozone, shippingoption,
  shippingoptionrule, shippingprofile, fulfillmentprovider, `POST /fulfillment/:id/ship|cancel`
- `tax` — taxregion, taxrate, taxraterule, taxprovider, `POST /tax/calculate`
- `customergroup` — customergroup + `/:id/members`, customergroupmembership
- `apikey` — publishableapikey, role, apipermission
- `notification` — notification

### Net-new native domains
- **Gift cards** (`models/giftcard`, `models/giftcardredemption`, `api/giftcard`):
  `gift-card` code+initial balance; balance is a PROJECTION `initial − Σ redemptions`
  (no mutable counter to race). `POST /giftcard/:id/redeem|void|` + `GET /balance|redemptions`.
  Admin-gated. Redeem is idempotent (see money design below).
- **B2B** (`models/company|employee|quote|approval`, `api/b2b`): company →
  employees (spending limits, `RemainingSpendCents`/`WithinLimit` pure fns) →
  quotes (RFQ, `POST /:id/accept|reject`, message thread) → approvals
  (`approval.NextStatus` pure transition, `POST /:id/approve|reject`).
- **Order exchange** (`models/exchange`, `api/exchange`): return + replacement,
  `DifferenceDueCents`, `POST /exchange/:id/confirm|cancel`.
- **Idempotency guard** (`models/idempotencykey`): reusable `Begin/Complete`
  for money-moving requests; wired into `api/checkout` `Refund` via
  `X-Idempotency-Key` (replay returns stored response; in-flight → 409).

### Platform product catalog — commerce is the CMS SOT (1.42.41)
`models/catalogentry` (`catalog-entry`, kind 273) is the source-of-truth for the
platform product catalog (Models, Vector, KMS, …) — the list docs.<brand> + the
console sidebar + pricing all derive from. **commerce owns the DATA (source +
seed + edits); `@hanzo/products` (hanzoai/ui/pkgs/products) owns the SCHEMA** +
the iconKey→component and brandColor→css code-maps. Conform to its CatalogEntry
exactly — the shape is NOT ours to change.
- **`GET /v1/commerce/catalog?brand=<b>`** (public, no auth, on the commerce
  public group so it serves that exact path) → `{brand, categories[], products[]}`;
  `products` is the `@hanzo/products` CatalogEntry[] (the client reads `.products`,
  tolerating a bare array or `{catalog|data|items}` too).
- CatalogEntry contract fields (`models/catalogentry/projection.go` Item): `id`
  (== slug, unique), `name`, `category` (EXACTLY one of the 13 canonical labels),
  `brandColor` (a swatch KEY like "violet" — NOT hex; client maps key→css),
  `iconKey` (a lucide export NAME like "Brain" — NOT a component), `slug`,
  `route` ("/<slug>"), `docsUrl` ("https://docs.hanzo.ai/docs/services/<slug>"),
  `apiPath` (/v1-prefixed), `pricingId` (plans/<key>.json key OR **null** — emit
  `*string`, nil→null). `brands` is a category-DERIVED convenience, never a
  hand-authored filter. Extra fields (gcp/repo/admin/status/priceCents/order/
  productId) are additive — the client ignores unknowns.
- **Brand scope is by CATEGORY** (`categoriesForBrand`, mirroring `@hanzo/products`
  `catalogForBrand`/`BRAND_CATEGORIES`): hanzo=all 13; lux/zoo/pars=5
  (Web3/Network/Security/Dev/Settings). There is NO per-entry brand filter — the
  same `system`-namespace store serves every brand; an entry shows iff its
  category is in the requested brand's set. Slug is globally unique.
- **Admin CRUD** `/v1/catalog/entries` (+ `/seed`) gates on
  `auth.IAMClaims.GlobalAdmin()` — the catalog is cross-tenant PLATFORM data in
  the `system` namespace, so an org-level admin must NOT edit it. Keyed by slug.
- **Seed**: the embedded `seed/hanzo-catalog.json` is the `@hanzo/products`
  snapshot (`hanzoai/ui/pkgs/products/snapshot/catalog.json`) VERBATIM — 95
  products with correct brandColor/iconKey/route/docsUrl/apiPath/pricingId.
  `SeedIfEmpty` auto-runs on first boot (count-gated no-op once populated;
  `COMMERCE_CATALOG_SEED=false` to skip). Idempotent + non-destructive — never
  clobbers CMS edits. Re-baseline by re-copying the `@hanzo/products` snapshot.
- Namespace: platform-global (`system` ns), namespaced via CONTEXT
  (`nscontext.WithNamespace`) — struct `SetNamespace` is a no-op for queries (they
  read ns from `d.Context`), a latent gotcha `api/billing/osspayout systemDB`
  also trips (works only because its reads+writes are symmetric in default ns).

### Model catalog — the OpenRouter cost sync (models/catalogentry/{openrouter,sync}.go)

A model IS a `catalogentry` (`model.go`): same kind, same `system` store, same
super-admin CRUD, same margin projection. Price is a `Rate` VECTOR (`rate.go`) —
cost is what WE pay (synced), price is what the CUSTOMER pays (set in admin),
margin is derived. `UpsertModels` is the ONE write seam and enforces `costOnly`.
This section is the PULL half: reading a live upstream on a schedule.

- **`FetchOpenRouter` / `DecodeOpenRouter`** — split on purpose: the fetch is the
  only I/O, so every rule about what a payload MEANS is testable against
  `testdata/openrouter-models.json` (captured from live) with no network. Decode
  FAILS on a malformed or empty body rather than returning a partial read: a
  half-decoded catalog looks exactly like an upstream that dropped 200 models,
  and the sync's answer to that is to withdraw them, so the two must be told
  apart before that decision is reached.
- **Per-token → per-MTok is EXACT**: `dec(s).Shift(6)`, decimal all the way, no
  float. `0.000000003625` → `0.003625`. A component the upstream prices at ZERO
  is omitted, never stored as cost `"0"` (which would read as 100% margin); a
  fully free model keeps its row with no rates — it is still a model and this
  catalog lists every model.
- **Vendor is the PARTY, not party+product** — "Meta", never "Meta Llama".
  `resolveVendors` settles ONE name per NAMESPACE across the whole payload
  (per-model derivation gives "OpenAI" and "Openai" for one vendor). It votes on
  the upstream's OWN display prefix, so a vendor added tomorrow is named right
  with no edit; `vendorNames` corrects only the ~5 namespaces publishing no
  prefix. `~vendor/x-latest` aliases are listed and marked `floating` in metadata
  — the id does not pin a version, so its cost can move under a stable id.

**`Refresh(db, serves, rows)`** wraps `UpsertModels` with the two things a
scheduled PULL needs that a push of decided rows does not:

1. **Withdrawal sweep.** Upsert only sees what the upstream still publishes, so
   on its own a withdrawn model stays listed and routable forever. Refresh
   withdraws the difference: `Spec.Enabled=false` + `Spec.Unavailable` =
   `WithdrawnUpstream`. MARKED, NEVER DELETED — usage rows, invoices and old
   conversations reference it by slug. It stays published and priced. A later run
   that finds it restores it, but ONLY if WE withdrew it: an operator who
   disabled a model by hand keeps that decision.
2. **Fail-safe.** `ErrEmptyUpstream` — zero models is indistinguishable from an
   outage, and believing it would withdraw the whole catalog in one run. A failed
   sync writes nothing and the last good catalog stands.

The sweep is **scoped by `Spec.Serves`**, so an OpenRouter outage can never
withdraw an Enso or Zen model.

**`freezePrice` — why a new row is priced explicitly.** `RetailPrice` falls back
to cost × markup when no price is stored, which spares a 341-row long tail from
hand-pricing. But a price COMPUTED FROM A LIVE COST moves whenever the upstream
moves — raise their cost and every bill rises, decided by nobody. Refresh
therefore writes the retail price out explicitly at CREATION only: the row
arrives at cost × markup (so landing it changes no bill and nothing needs
hand-pricing), and from then on the price is a stored number only a human
changes, so the next upstream move surfaces as a MARGIN THAT MOVED in
admin.hanzo.ai. Creation is the only legitimate moment — the row is new, so
there is no customer relying on a price to disturb. NOTE: a row created by hand
without a price still tracks cost through the fallback; only synced rows are
frozen.

- **Route**: `POST /v1/catalog/models/refresh` (super-admin). `POST
  /v1/catalog/models` remains the PUSH of decided rows; both funnel through
  `UpsertModels`, so the cost/price rule holds whichever door a row came through.
  Scheduled by a CronJob curling it with `COMMERCE_SERVICE_TOKEN`, the same shape
  the contributor payout uses. It deliberately does NOT run at boot — a
  boot-time call to a third party is a boot hazard.
- **`/catalog/entries/*` is a WILDCARD, not `:slug`.** A model's slug IS its
  callable id and those contain a slash; a segment param stops at the slash, so
  every model row would be listed and un-editable. Measured on this router:
  fiber's `+` param does not bind, `*` does, and it matches a single-segment slug
  identically (infra-tier edits unchanged). Pinned by
  `TestAdminRoute_AddressesASlashBearingSlug`, which drives the real `AdminRoute`.
- **Proven against live OpenRouter**: 341 models created; a second run against
  the same payload creates, withdraws and restores nothing; halving the upstream
  withdrew 171 and deleted ZERO (all 341 rows still present); recovery restored
  171. claude-opus-5 lands at cost 5/25 → price 6/30 per MTok, margin 16.67% on
  each rate; the public projection carries price and no cost. `Refresh` is stable
  on STATE — `UpsertModels` still rewrites every row each run, so the write count
  is not yet a no-op.

### Money-correctness design (idempotency WITHOUT working transactions)
- `datastore.RunInTransaction` (`datastore/datastore.go`) is a **NO-OP** — it
  runs `fn(New(ctx))` with no tx/lock/isolation. `db.SQLiteDB.RunInTransaction`
  IS real (writeMu.Lock+BeginTx) but `mixin.Model[T]` routes to the no-op. So
  model-level read-modify-write is NOT atomic. The pre-existing Square refund
  (`api/checkout/square/refund.go`) is TOCTOU-vulnerable to concurrent
  double-refund (guards over-refund but not races/replays).
- The money-safe primitive used here: **deterministic-id ledger records**. A
  redemption/guard's STORAGE id = `sha256(scope‖key)` via `orm.WithStringKey`.
  Concurrent duplicate submits collapse onto ONE row via the storage
  `ON CONFLICT(id,kind,namespace)` upsert — proven (giftcard 25-goroutine
  same-key test → 1 debit). Balance is a projection so distinct concurrent
  redemptions are additive, never lost. Refund guard replays the stored
  response. The narrow concurrent-first-submit window is closed at the gateway
  (Square/Stripe honor the same idempotency key on the refund call).
- **GOTCHA — a struct field named `Key` SHADOWS `Model[T].Key()`** and silently
  breaks `mixin.Entity` (`.Query()` returns nil entity → nil panic in `Get`).
  Name idempotency fields `IdemKey`, not `Key`. Compiler catches it only via an
  explicit `var _ mixin.Entity = (*T)(nil)` assertion.
- **GOTCHA — namespace scoping keys off CONTEXT, not the datastore struct.**
  `db.SetNamespace(ns)` sets `d.namespace` but the SQL layer reads
  `nscontext.GetNamespace(ctx)` (`db/query.go getNamespace`). Production is
  correct (`org.Namespaced(c)` → `nscontext.WithNamespace`); tests MUST build
  the datastore from a namespaced CONTEXT (`datastore.New(nscontext.WithNamespace(ctx, ns))`),
  not `SetNamespace`, or cross-tenant isolation appears broken in tests only.
- `datastore/query/query.go` `ById` switch: all hashid-only `Model[T]` kinds
  (no slug/name secondary lookup) MUST be in the "return not-found" case, else a
  non-hashid id hits `default:` → HTTP 500 instead of 404. 43 such kinds were
  missing and are now listed.

### Endpoint contract note (console → /v1/commerce)
The model bundle serves `/v1/product` etc. (bare `/v1`). console2 today reaches
it via a same-origin `/commerce` BFF proxy (`app/commerce/[...path]/route.ts` →
`commerce.hanzo.svc:8001/v1/*`). To honor the "console calls `/v1/commerce/*`,
nothing before /v1/" law without breaking the live `/v1/billing/*` money path,
add an ingress rewrite on the console host (`console.hanzo.ai/v1/commerce/* →
commerce/v1/* strip`) rather than re-prefixing the backend bundle.

- Healthcheck: use `curl -f` not `wget --spider` (Gin only handles GET)
- `go-sqlite3` must be pinned to v1.14.x (`replace` in go.mod) — the transitive
  v2.0.3+incompatible fails to compile on musl/Alpine (pread64/pwrite64)
- Meilisearch v0.35.1 changed `AddDocuments`/`DeleteDocuments` signatures
- Production Dockerfile uses CGO_ENABLED=1 for SQLite (not CGO_ENABLED=0)
- Global entities (Organization, User, Token) use `DefaultNamespace = true` by design
- PVC is ReadWriteOnce — use Recreate deployment strategy, not RollingUpdate
- Filter field names are PascalCase (Go struct) — auto-converted to camelCase JSON
- Boolean `false` with `omitempty` may be omitted from JSON — COALESCE handles NULL=false

## Un-fork: THIS repo is canonical; cloud imports it (v1.47.0)
Cloud's inlined fork (`hanzoai/cloud clients/commerce/`, no go.mod) is being
retired: cloud `import github.com/hanzoai/commerce` + a thin subsystems adapter.
v1.47.0 = content parity with the fork's last cloud-side deltas:
- product.created/updated events (`events/`, `api/api/product_events.go`) — storefront loop
- `middleware/accesstoken.go`: service-token checked BEFORE the IAM branch (S2S
  metering dispatch must never hit the per-user scope gate) + `ensureIAMOrg`
  (org from gateway X-Org-Id via svcorg.Resolve; IAM is the one org authority)
- sessionless webhook fixes: `resolveWebhookOrg` uses GetOrganizationOK (no
  panic on provider deliveries); square provider seeds from env at init so
  webhook validation works with no tenant resolver in front
- `Embedded.brand` (set by the mounter; surfaced in the in-process OrgConfig)
- v1.47.1: `api/api` flattened to `api` (package api at api/; the old GAE-era
  api/main.go binary moved to cmd/api) + the service-token-precedence regression
  tests live HERE with the accesstoken fix they pin
NOT ported (deliberate): the fork's GCP-style BillingAccount
(Default/LimitCents/projectbinding budget cluster) — the money-of-record
BillingAccount + Binding + resolveBilling chain HERE is the one design; the
fork's spend-cap/budget features must be re-expressed on Binding (#70), not
imported as a competing model. Also not ported: thirdparty/ethereum (dropped
deliberately, LGPL), and the cloud bridges (finance-coupled metering client,
in-process entitlement client, Deps adapter) — those live cloud-side.

## v1.48.0 — NATIVE zip (zap-proto/fiber): gin is GONE
Whole repo runs on zap-proto/zip v1.7.5 (zap-proto/fiber underneath). No gin,
no gofiber, no net/http adaptation in the serving path. Handlers are
`func(*zip.Ctx) error`; returning the render IS the abort (no gin Abort()).
- ONE request context: `middleware.RequestContext()` does `c.SetContext(mintauth.WithGate(...))`
  at the boundary; everything reads `c.Context()`. `GetContext`/`Locals("context")` are DEAD.
  `middleware.AuthorizeMint` SetContexts the authorized ctx — the ledger sink reads it there.
- Route chains are gin-order (middleware…, handler LAST) — zip ≥v1.7.4 executes
  them in that order (v1.7.4 fixed an inversion; chain_order_test pins it).
- Empty leaf = group root (zip ≥v1.7.5 normPath); fiber param names are dash-free
  (rest.ParamId squashes dashes: kind "product-option" → param "productoptionid").
- Co-residence contract: `EmbedConfig.App *zip.App` / `Config.SharedApp` — commerce
  registers routes ON the host's app (one router, one specificity space);
  standalone-only surfaces (healthz, legacy /admin SPA, checkout SPA catch-all,
  Listen/TLS) are skipped when embedded. `Embedded.HTTPHandler/HTTPAddr` DELETED → `Embedded.Zip()`.
- SPA serving: zip.Static(+WithIndex/WithFallback) for admin (ui.FS); checkout
  SPAHandler is a native zip handler (same security headers).
- Test idioms: zipclient (ex ginclient) drives via the fiber adaptor with
  SERVER-style requests; PostForm encodes the body (fasthttp parses bytes, not
  req.PostForm). zip TestCtx for direct handler calls.
- Pre-existing debt unchanged: test-integration/* needs external services;
  thirdparty/reamaze verifyHMAC has an inversion bug (flagged, not fixed here).

## Licensing — `MIT OR Apache-2.0` (HIP-0137)

The repo is dual-licensed per **HIP-0137 "One License"**
(`hanzoai/hips`, `HIPs/hip-0137-one-license.md`). Three root files, one
declaration each, no restating:

- `LICENSE` — the dual offer + SPDX id + inbound=outbound contribution clause.
  Points at the two texts and at NOTICE. Copyright `2014-2026 Hanzo AI, Inc.`
  (the real span: root commit is 2014-09-29, this started life as `kaching`).
- `LICENSE-MIT` — canonical MIT, Hanzo copyright line.
- `LICENSE-APACHE` — canonical Apache-2.0, BYTE-IDENTICAL to
  apache.org/licenses/LICENSE-2.0.txt. sha256
  `cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30`, 11358 bytes,
  git blob `d645695673349e3947e8e5ae42332d0ac3164cd7`. Verify with `sha256sum` /
  `cmp` / `git hash-object` — never `diff` (it is a shell function on the dev box
  and has reported "differs" on byte-identical files). Any alteration to operative
  text means the file is no longer that licence.

Every `package.json` in the tree (15 of them, all under `app/`) declares
`"license": "MIT OR Apache-2.0"`. There is no Cargo.toml or pyproject.toml here,
and go.mod has no licence field, so the manifests are the whole surface.

`NOTICE` is the third-party attribution file — it scopes *vendored dependencies*,
not this repo's own terms. Its btcd/btcec (ISC, `replace/btcec`) and Go stdlib
(BSD-3) sections are untouched. Appended: **Medusa** (medusajs/medusa, MIT,
Copyright (c) 2021 Medusajs), which was vendored but unattributed. The whole
TypeScript frontend under `app/` is rebranded Medusa — `app/admin` is
`@medusajs/dashboard` (its CHANGELOG.md still carries that header), `app/site` is
the Medusa Book (upstream Cloudinary "Medusa Book" asset URLs are still inline),
`app/packages/*` are the corresponding `@medusajs` packages, and `app/store`
derives from medusajs/nextjs-starter-medusa (MIT, Copyright (c) 2022 Medusa; its
yarn.lock still resolves `medusa-next@workspace:.`). MIT grants sublicensing, so
offering the whole under `MIT OR Apache-2.0` is sound *provided* the upstream
copyright notice is retained — that retention is what the NOTICE append is for.
Upstream ships no NOTICE file, so nothing further propagates.

The **Go backend is not a Medusa fork** — it is an independent native
implementation that reaches parity with Medusa v2's admin domains (see "Medusa
parity" above). Only `app/` carries upstream lineage.

### `app/store/LICENSE` — a swapped copyright line, REPAIRED

That file was the upstream starter's MIT licence with line 3 changed from
`Copyright (c) 2022 Medusa` to `Copyright (c) 2024 Hanzo AI`. Every other byte
matched upstream. It is restored: the file is now byte-identical to
medusajs/nextjs-starter-medusa's LICENSE — sha256
`770a61c897d73cdeb53ff2a367d537a80eccf57b31f81e4a5fe951fecc40c632`, git blob
`94def62e7e4e5016c3c4af42bc426beb3ec483e8`, which is the blob id GitHub reports
for the upstream file itself.

**The rule is "never edit an upstream LICENSE," and restoring this is not an
exception to it — it is the rule being enforced.** The rule exists to protect
upstream attribution. This was not an intact upstream licence being altered; it
was an already-altered one, altered in the single way MIT forbids: MIT
conditions the grant on "the above copyright notice ... shall be included in all
copies or substantial portions of the Software." A swapped holder line is a
continuing breach for as long as it ships. Restoring is repair, not edit. The
test to apply next time: *does this change move the file toward or away from
what upstream published?* Toward = repair. Away = the thing the rule forbids.

This is the second confirmed instance of this failure class, after the Papermark
incident (a branding pass retitled Papermark's commercial licence and swapped
its copyright holder over 100% upstream code). Both were produced by
rebrand-everything sweeps that treated a licence file as branding surface.
Licence and NOTICE files are not branding surface.

Swept the rest of the repo for the same swap. Tracked licence files repo-wide
are exactly five: the three root ones, NOTICE, and `app/store/LICENSE` — no
others, nested or otherwise (the `node_modules/` and `.next/` hits a filesystem
grep returns are untracked build output, 0 tracked). `app/store/LICENSE` was
also the **only** Hanzo copyright claim anywhere in the vendored `app/` tree: no
Medusa source file was stamped with a Hanzo header. The vendored btcd/btcec
under `replace/` keeps its ISC notices in-file (22 of 44 files carry the
header; the rest are go.mod/go.sum/README/testdata).

### Per-file Go headers — all 72 now agree with the root

Every Hanzo-authored Go file carries the same two-line header, and it says the
same thing the root LICENSE says:

```go
// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.
```

That replaced two stale forms: `// Copyright © 2026 Hanzo AI. MIT License.` (71
files, understated the dual offer) and, in `cmd/commerced/telemetry.go`,
`// Copyright 2023-2026 Hanzo AI Inc. All Rights Reserved.` (1 file, a
proprietary reservation left over from before the relicense). Headers are what
licence scanners actually read, so a header contradicting the root is the
contradiction that gets found first.

The span is `2014-present` — one string, everywhere, matching `LICENSE`,
`LICENSE-MIT` and `NOTICE`. It is the true span: the root commit is `1a1273cc2`,
2014-09-29, when this was `kaching`. The old `2026` was wrong on its own terms,
post-dating files that had existed for a decade. Do not reintroduce a per-file
year convention — nobody maintains one, and it drifts from the root within a
release.

Six of the 72 have a `//go:build` constraint below the header. The rewrite
replaced line 1 in place rather than inserting, so the constraint keeps its
position behind blank-lines-and-line-comments only — the one way this breaks
silently. `go vet ./...` is the gate that proves it (its buildtag analyzer
flags a misplaced constraint); it is green, as is `CGO_ENABLED=0 go build ./...`.

Nothing vendored was restamped: the only other Go files carrying a copyright
line are the 33 under `replace/btcec`, which keep their upstream btcsuite and
Decred ISC notices verbatim. If a file's header is upstream's, it is not ours to
restamp — `git ls-files` the path before touching it, and note that
`app/node_modules/` and `app/store/.next/` are untracked build output, not repo
content.

Build note unrelated to licensing: plain `go build ./...` fails under cgo with
`undefined: cgoBuildNeedsSQLiteMathFunctions` from `hanzoai/base`. That is a
deliberate compile-time guard in the dependency (`//go:build cgo &&
!sqlite_math_functions`) telling you to use the pure-Go backend or add the tag.
It reproduces identically on a clean tree, and `CGO_ENABLED=0` — what the
Dockerfile ships — is green. Not caused by the headers.

commerce stays PRIVATE. Licensing and visibility are independent: a gitleaks
sweep of full history found live third-party merchant credentials and a
customer's production hostnames. A licence change does not rewrite history.
