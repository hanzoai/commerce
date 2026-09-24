# Commerce Admin (`@hanzo/commerce-dashboard`)

The merchant admin console for **commerce.hanzo.ai** — the store dashboard where a
merchant onboards a store, manages products, orders, customers, collections and
inventory, configures tax/regions/sales-channels, and self-manages billing.

It is a **Next.js app in `output: 'export'` mode** (a static site, no Node server
at runtime). The export is built from source by `Dockerfile.commerce-admin` and
served by the canonical Hanzo static plugin (`ghcr.io/hanzoai/static`) — **never
nginx, never a node server**.

> The live shell is `src/app` (Next app-router). The older React Router tree under
> `src/routes` is included-but-unreachable Medusa-lineage code (a parts bin), not
> the shipped app. Do not confuse the two.

## Develop

```bash
# from the FE pnpm workspace root (commerce/app)
pnpm install

# run the admin dev server (Next dev, hot reload)
pnpm --filter @hanzo/commerce-dashboard dev     # http://localhost:3000
# or, from this directory:
#   next dev
```

Build-time config is read from `NEXT_PUBLIC_*` env vars, which Next inlines into
the client bundle. Point them at a local or staging backend during development:

| Var | Purpose | Prod default |
|-----|---------|--------------|
| `NEXT_PUBLIC_COMMERCE_API_URL` | Commerce API origin | `https://commerce.hanzo.ai` |
| `NEXT_PUBLIC_IAM_SERVER_URL` | OIDC issuer | `https://hanzo.id` |
| `NEXT_PUBLIC_IAM_CLIENT_ID` | PKCE client id | `hanzo-commerce` |
| `NEXT_PUBLIC_HANZO_AI_URL` | AI dock chat-completions gateway | `https://api.hanzo.ai/v1/chat/completions` |
| `NEXT_PUBLIC_HANZO_AI_MODEL` | AI dock model | `best` |

## Build

```bash
# produces the static export in ./out (out/<route>/index.html)
pnpm --filter @hanzo/commerce-dashboard build
# or, from this directory:
#   next build
```

`next.config.ts` pins the export contract:

- **`output: 'export'`** — a fully static site (no SSR/ISR at runtime).
- **`trailingSlash: true`** — directory-style export (`out/overview/index.html`),
  so a deep-link or hard refresh on a route resolves through the static server's
  directory index. A flat `overview.html` (SPA-only) would serve the landing page
  for every deep link.
- **`images: { unoptimized: true }`** — no image optimizer in a static export.
- **`transpilePackages`** — the workspace UI/SDK packages (`@hanzo/commerce-ui`,
  `-icons`, `-sdk`, `-types`, `-admin-shared`) are compiled in.
- **`typescript.ignoreBuildErrors` / `eslint.ignoreDuringBuilds`** — the vendored
  Medusa-lineage tree (`src/components/data-grid`, `src/routes`, …) is included but
  unreachable and carries upstream strict-TS/lint noise that never enters the
  bundle. Authored code is type/lint-checked separately (`pnpm test`, `pnpm lint`);
  webpack still fully compiles everything the app-router actually reaches.

## Deploy

Two-stage container, defined by **`Dockerfile.commerce-admin`** (built from source
by CI — never on a laptop):

1. **build** — `turbo run build --filter=@hanzo/commerce-dashboard` inside the FE
   pnpm workspace, with the `NEXT_PUBLIC_*` prod defaults baked in at build time.
2. **serve** — copy `app/admin/out` into `ghcr.io/hanzoai/static` and run
   `static --root=/srv --spa --port=3000`.

Chunks are root-relative (`/_next/*`, no `assetPrefix`), so the static tree serves
at root directly. The operator CR points the admin host at this image.

## API convention (`/v1`, same-origin)

The admin talks to the commerce backend over the platform **`/v1`** REST surface —
no `/api/` prefix. In production the admin and the API are **same-origin** on
`commerce.hanzo.ai`: the ingress routes `/v1`, `/api`, `/healthz` to the commerce
API (`commerce.hanzo.svc:8001`) and everything else to this static tree.

- Products use the bare resource `/v1/product`. The Products form and the AI
  `create_product` command both call the same `createOne("product", …)` path.
- Billing/self-service reads hit `/v1/billing/*`.
- Integrations read/toggle `/v1/c/:org/integrations`.
- Catalog (platform products) is `/v1/commerce/catalog`.

Identity is **native IAM PKCE** through `@hanzo/iam` against `hanzo.id`. The active
organization (the org switcher) scopes every request; it is the sole tenant
selector. The browser never holds provider secrets — those live in KMS and only
become toggleable once their credentials exist.

## In-cloud note (`commerceinproc`)

In production the commerce **backend** runs **in-process inside the single cloud
binary** (HIP-0106 unified cloud binary; see `../../docs/architecture.mdx`). The
cloud host mounts commerce's routes on its own router and reaches the metering /
billing handlers by a direct Go call over `commerceinproc` — no socket to
`commerce.hanzo.svc:8001`. This admin (a static browser app) is unaffected: it
always talks to the same `/v1` HTTP surface at `commerce.hanzo.ai`, whether the
backend is standalone or co-resident. The identity edge (`SanitizeIdentity` in the
cloud host) mints the trusted `X-Org-Id` / `X-User-IsOrgAdmin` / `X-User-IsAdmin`
headers the backend reads — the browser never sets them.
