# metering — the one way to meter usage to commerce

`github.com/hanzoai/commerce/metering` is the single, reusable hook every Hanzo
product uses to charge for usage. It is the DRY replacement for the balance-check
and usage-record logic that was copy-pasted across the LLM/cloud path
(`ai/routers/filter_balance.go`, `ai/controllers/openai_api.go`,
`gateway/auth_middleware.go`). Any product — search, functions, storage, a CLI —
imports this package and gets the proven, fail-closed billing gate plus usage
recording, against **commerce, the single billing source of truth**.

## What it does

Two operations, matching the proven cloud/gateway path:

| Op | Commerce endpoint | Purpose |
|----|-------------------|---------|
| `Authorize` | `GET /v1/billing/balance`, then `GET /v1/billing/alerts/authorize` | Pre-request balance + spend-cap gate. **Fail-closed, always.** |
| `Record`    | `POST /v1/billing/usage` | Post-request usage write. Every served call writes a row, `$0` included. |

Auth is the commerce service token (admin-scoped S2S):

```
Authorization: Bearer ${COMMERCE_SERVICE_TOKEN}
X-Org-Id: <tenant org slug>
```

The token is a secret and **must** come from KMS (the operator wires it from a
KMS-backed secret into `COMMERCE_SERVICE_TOKEN`). This package never reads it
from disk.

## Use it (middleware — the common case)

```go
meter, _ := metering.FromEnv() // COMMERCE_URL + COMMERCE_SERVICE_TOKEN (KMS) + COMMERCE_SERVICE_ORG

mux.Use(meter.Middleware(metering.MiddlewareConfig{
    Provider: "search",
    Price: func(r *http.Request, status int, in metering.AuthInput) int64 {
        if status >= 200 && status < 300 { return 5 } // 5¢ per successful request
        return 0 // counted, not charged
    },
    Skip: func(r *http.Request) bool { return r.URL.Path == "/healthz" },
}))
```

`Middleware` is plain `func(http.Handler) http.Handler`, so it composes with the
standard library, `gorilla/mux` (`.Use`), `chi`, and anything that speaks
`http.Handler` — no per-framework variants.

It reads the caller identity from the **gateway-minted** `X-User-Id` / `X-Org-Id`
headers (the trust boundary). The gateway strips client-supplied copies on
ingress, so these are safe to trust downstream.

## Use it (imperative — per-unit pricing)

```go
in := metering.IdentityFromGatewayHeaders(r)
if err := meter.Authorize(ctx, in); err != nil {
    // ErrInsufficientBalance / ErrSpendCapExceeded -> 402 ; other -> 503
}
// ... do work, measure cost ...
meter.Record(ctx, metering.Usage{User: in.User, Org: in.Org, AmountCents: cents, Provider: "functions"})
```

## Configuration (env, operator-wired)

| Var | Default | Notes |
|-----|---------|-------|
| `COMMERCE_URL` | `http://commerce.hanzo.svc.cluster.local:8001` | Commerce base (no `/v1` suffix). |
| `COMMERCE_SERVICE_TOKEN` | — | Admin-scoped S2S token. **KMS-sourced.** |
| `COMMERCE_SERVICE_ORG` | `hanzo` | Default tenant org (`X-Org-Id`). |

No variable turns the gate off, opens it on error, or routes it to a sandbox
ledger.

## Fail-closed contract (aligned with the gateway)

`Authorize` returns:

- `nil` → allow.
- `ErrInsufficientBalance` → out of funds → **402**.
- `ErrSpendCapExceeded` → funded, over a scope's cap → **402**.
- any other error → balance or cap unknown → **deny** → **503**.

`New` refuses an empty `COMMERCE_URL`, and a nil `*Client` refuses every call
with `ErrNotConfigured` (503): nothing is served unchecked. A `Middleware` with
no `Price` refuses every metered request.

This is the same balance source and the same status mapping the gateway uses —
no divergent logic.
