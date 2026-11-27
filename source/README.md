# `source/` — Multi-Store Bidirectional Storefront Connectors

`source/` implements Hanzo Commerce's multi-store bidirectional sync engine (`1 Org : N Connected Stores`), mounted under `/v1/source/*`.

## Packages

| Package | Responsibility |
| --- | --- |
| `source/hub` | `StoreSource` registry keyed by `(org_id, source_id)`, AES-256-GCM HKDF-sealed per-source credentials, loop-free `SyncLedger` (`last_synced_hash` / `pending_outbound_hash`), master SKU cross-store inventory unification, and outbound sync queue |
| `source/shopify` | Server half of `hanzo/shopify`: App Home (`/app`), App Bridge HS256 JWT verification (`/session`, `/link`, `/pixel`), IAM link callback (`/callback`), HMAC-verified webhooks (`/webhook`, mandatory compliance topics returning `401` on bad HMAC), Flow action & trigger (`/flow`), and Admin GraphQL `2026-10` outbound driver |
| `source/woocommerce` | Server half of `hanzo/woocommerce`: per-shop secret connect (`/connect`), `Client::sign` HMAC-SHA256 ingest/sync/webhook (`/ingest`, `/sync`, `/webhook`), GDPR export & erase (`/export`, `/erase`, including `guest:<order_id>`), and bidirectional outbound push/pull (`/outbound`) |
| `source` | Composition bundle & REST observers (`ObserveCouponREST`, `SyncCanonicalProductByID`) |

## Endpoints

- `GET|POST /v1/source/sources`, `GET|PATCH|DELETE /v1/source/sources/:sourceid`, `POST /v1/source/sources/:sourceid/reconcile`
- `POST /v1/source/publish/:kind` (`product`, `coupon`, `inventory`, `fulfillment`, `refund`, `consent`, `campaign`)
- `GET /v1/source/shopify/app`, `POST /v1/source/shopify/{session,link,pixel,webhook,flow}`, `GET|POST /v1/source/shopify/callback`
- `POST /v1/source/woocommerce/{connect,ingest,sync,webhook,export,erase,outbound}`
