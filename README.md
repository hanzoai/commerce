<p align="center"><img src=".github/hero.svg" alt="commerce" width="880"></p>

# commerce

Checkout, billing, pricing, invoicing — the light commerce router for the Hanzo platform. **NOT in PCI-DSS scope** (PAN data lives in `hanzoai/vault`).

[![Status](https://img.shields.io/badge/status-stable-green)]()
[![License](https://img.shields.io/badge/license-MIT%20OR%20Apache--2.0-blue)]()

## Quick start

```bash
docker run -p 8001:8001 ghcr.io/hanzoai/commerce:latest
```

## What this is

`commerce` handles orders, products, subscriptions, invoices, and customer records for every Hanzo deployment. It is **CDE-connected, not CDE-in-scope**: card data is tokenized at `hanzoai/vault` and `commerce` only ever sees the token. The service is production multi-tenant (37+ `X-Org-Id` call sites), IAM-integrated, and already exposes `func Mount(*zip.App, cloud.Deps) error` — it is one of the three reference implementations of the HIP-0106 Mount contract.

## Specs

Implements:
- HIP-0037 AI Cloud Platform (billing surface)
- HIP-0106 Unified Cloud Binary (commerce subsystem — already exposes `Mount()`)

## Architecture

```
   client  ->  gateway  ->  commerce (zip.App)
                              |
                     orders / products / subscriptions / invoices
                              |
                  +-----------+-----------+
                  |                       |
              hanzoai/vault          billing provider
              (CDE — tokenizes PAN)   (Stripe etc., webhook in)
                              |
                  per-tenant data via base (HIP-0302)
```

## Development

```bash
sh scripts/fetch-plans.sh                  # vendor the @hanzo/plans catalog (go:embed)
go run ./cmd/commerce -dev                  # API on 127.0.0.1:8090
```

Data lives under `COMMERCE_DIR` (default `./commerce_data`), one SQLite file per
org. The build needs cgo with the `sqlite_math_functions` tag, or
`CGO_ENABLED=0` for the pure-Go SQLite backend.

## Test

```bash
export GOWORK=off CGO_ENABLED=1 GOFLAGS=-tags=sqlite_math_functions
go vet ./...
go test ./billing/... ./payment/...
go run github.com/onsi/ginkgo/v2/ginkgo -r test/
```

The same gates run in CI from `hanzo.yml`.

## License

`MIT OR Apache-2.0` (`LICENSE`, `LICENSE-MIT`, `LICENSE-APACHE`) per HIP-0137 One License; vendored third-party terms are in `NOTICE`.
