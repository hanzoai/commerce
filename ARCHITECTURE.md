# Architecture

How `commerce` is put together, and the invariants a change must not break.

This is the contributor's map. It documents the software; it deliberately says
nothing about how any particular deployment is configured or operated.

## What it is

A multi-tenant commerce and billing engine: one Go binary, embedded SQLite by
default, PostgreSQL and ClickHouse available. It serves orders, products,
subscriptions, invoices, plans, usage metering and a credit ledger.

It runs two ways from the same code:

- **standalone** — `commerce serve` owns the process and the HTTP listener.
- **embedded** — a host binary calls `Mount(*zip.App, cloud.Deps) error` and
  commerce registers its routes on the host's router. There is one router and
  one route-specificity space; standalone-only surfaces (health, TLS, the SPA
  catch-all) are skipped when embedded.

Handlers are `func(*zip.Ctx) error` on [zip](https://github.com/zap-proto/zip).
Returning the render **is** the abort — there is no separate abort call.

## Layout

```
api/            HTTP handlers, one package per domain (billing, checkout, plan…)
billing/engine/ the billing lifecycle: invoices, renewals, usage, payouts
billing/bucket/ pure classifier: which money is credit vs prepaid
models/         persisted entities, each on mixin.Model[T]
payment/        provider registry + per-provider adapters
middleware/     identity, namespace, token gates
datastore/      the query layer over the storage backends
db/             SQLite / Postgres / ClickHouse backends, tenant store registry
```

## Multi-tenancy

An organization's name **is** its namespace. `middleware.Namespace()` sets it on
the request context and every downstream datastore read inherits it.

Two rules that are easy to get wrong:

- **Namespace scoping keys off the CONTEXT, not the datastore struct.**
  `db.SetNamespace(ns)` sets a field the SQL layer never reads. Build the
  datastore from a namespaced context —
  `datastore.New(nscontext.WithNamespace(ctx, ns))` — or tenant isolation looks
  broken in tests while being correct in production.
- Cross-org access gates on a verified global-admin claim only. An org-level
  admin role is scoped to its own org and is never sufficient.

## Money correctness

`datastore.RunInTransaction` is a **no-op** — it runs the callback with a fresh
datastore and no isolation. Model-level read-modify-write is therefore not
atomic, and nothing may rely on it being so.

The primitive used instead is the **deterministic-id ledger record**: a
record's storage id is `sha256(scope‖key)`. Concurrent duplicate submits
collapse onto one row through the storage upsert, so a double-submit debits
once. Balances are computed as a projection over ledger rows rather than held
as a mutable counter, so distinct concurrent writes are additive and none is
lost.

Money-moving HTTP requests carry `X-Idempotency-Key` and go through
`models/idempotencykey`'s `Begin`/`Complete`: a replay returns the stored
response verbatim, an in-flight duplicate gets a 409. The payment processor is
sent a key derived from the same stable value, so the charge is exactly-once at
the processor even if the local guard is unavailable.

**One discount arithmetic.** A subscription's charge is computed in `api/billing`
and its invoice in `billing/engine`. They must agree to the cent, so the
percent-off math lives in exactly one function, `engine.DiscountCents`, which
both call. A promo is captured on the subscription at signup and carried for its
life, so a renewal charges the deal the customer agreed to.

**Three money buckets.** `billing/bucket` is the one classifier: granted credit
and real prepaid money are distinct, and an unknown deposit is treated as
prepaid (fail-closed — never mint spendable value from an unclassified row).
Non-GPU usage draws credits first then prepaid; GPU usage draws prepaid only.

## Pricing

The plan catalog is a published package, vendored at build time and embedded in
the binary. On boot the live plan rows are reconciled to it. Changing a price is
therefore a versioned, reviewable, revertible deploy — not a mutation against a
running system. An operator edit to a plan is respected and not reverted by the
next boot; that is what distinguishes an admin decision from catalog drift.

Plans are archived, never deleted. `plan.Listed()` is the one predicate gating
both the public catalog and every purchase entrypoint, so retiring a tier stops
new sales without stranding existing subscribers — a renewal on a retired tier
must still be able to price itself.

## Credentials

The binary reads its provider credentials from a secret store at request time
and never persists them. There is no in-repo fallback and no default credential:
an unconfigured provider fails closed rather than transacting on someone else's
account. Configuration is by environment variable; see `deploy/.env.example`.

## Gotchas

- **Filter field names are Go PascalCase and convert to camelCase JSON.**
  `Filter("DestinationKind=", k)` becomes `json_extract(data,'$.destinationKind')`.
- **A struct field named `Key` shadows `Model[T].Key()`** and silently breaks the
  entity interface. Name idempotency fields `IdemKey`. Assert
  `var _ mixin.Entity = (*T)(nil)` to catch it at compile time.
- **A row from `GetAll` is a value with no storage binding.** Calling `Update()`
  on it writes nothing *and returns no error*. Re-load through the bound point
  query before writing.
- **New ORM kinds must be registered** in `util/hashid/kind.go` — monotonic,
  never reordered — or `Create()` panics.
- Route chains run middleware-first, handler-last.
- A cgo build needs `-tags sqlite_math_functions`. `CGO_ENABLED=0` is the
  path the container image builds.

## Building

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go test ./...
go run cmd/commerce/main.go serve --dev
```

## Licence

`MIT OR Apache-2.0`. See [LICENSE](LICENSE). Contributions are inbound=outbound
under the same dual terms. Vendored third-party components keep their own
notices — see [NOTICE](NOTICE).
