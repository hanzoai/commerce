# Hanzo Agent Commerce — design against what commerce already is

Status: PROPOSAL. Nothing here is built. This file maps the Hanzo/Lux split onto
this repo as it stands (2026-09) and names the first slice worth building.

## The split

| Lux (protocol, neutral, open) | Hanzo (commerce, compliance, ledger) |
|---|---|
| agent identity primitives, parent/child lineage | `EconomicPrincipal` registry, KYC/KYB |
| x402 payment intents | W-9 / W-8 / TIN collection, tax documents |
| wallet-to-wallet settlement | DAC7 / UK / CA / AU seller reporting |
| attestations, noncustodial execution | sanctions screening, fiat payouts, disputes |
| SDKs | reputation, invoices, accounting ledger, rules engine |

Hanzo owns **who is paying whom, and what that obligates.** Lux owns **how value
moves and who signed it.** The boundary between them is one value: a Lux
attestation that says agent `A` (lineage `root → … → A`) acts for key `K`.
Hanzo's only job at that boundary is to map `K` to an `EconomicPrincipal`.

This repo is the right home for the Hanzo half. It already has a
money-of-record ledger, a per-org tenancy model, an x402 implementation, a
crypto deposit/custody rail, payouts, disputes and tax models. What it lacks is
the two primitives that sit above them all.

## What exists, and what state it is in

| Responsibility | In this repo | State |
|---|---|---|
| x402 | `payment/x402` (protocol, client, facilitator, paywall middleware) | **Zero callers outside the package.** Local `Settle` goes through `MPCProcessor.Charge`, which now refuses unconditionally (`NOT_A_WALLET`), so only the remote-facilitator path could ever succeed. |
| Accounting ledger | `models/transaction` + `billing/bucket`, `billing/creditledger` | Real, idempotent, per-org. Settlement-rail agnostic already. |
| Crypto settlement | `billing/depositwatch`, `depositledger`, `billing/custody` | Receive + credit built. Sweep built, not called. `cryptoDepositsCanBeCredited == false`. |
| Fiat payouts | `models/payout`, `models/partner`, `api/payables` | Payout record exists; it has no principal, no tax classification. |
| KYC | `models/user.KYCData` | Legacy (tokensale era). Status + documents on the USER, not on a legal entity. |
| TIN | `user.KYCData.TaxId`, `partner.Partner.TaxId` | **Plaintext string fields**, and per-user stores are unencrypted (see CLAUDE.md, "At-Rest Encryption"). Must not become the W-9 store. |
| Tax | `models/tax*`, `api/tax` | SALES tax (Medusa parity). Nothing about information returns / seller reporting. |
| Disputes | `models/dispute` | Card-network dispute shape (Stripe statuses). Not a marketplace dispute. |
| Sanctions, DAC7, 1099, W-8, reputation | — | Nothing. |

So: rails yes, ledger yes, **principal and event no.** That is the right order
to be missing them in; it means nothing has to be torn out.

## Primitive 1 — `EconomicPrincipal`

The legal/tax party. Not an IAM org, not a user, not an agent.

- **IAM org ≠ principal.** An org is a TENANT (namespace, isolation). A
  principal is a LEGAL PARTY (tax residency, TIN, bank account). An org usually
  has one; a platform org paying out to thousands of sellers references thousands
  it does not own. Collapsing them repeats the "three auth systems" misread in
  CLAUDE.md in a new place.
- **Lives in the `system` namespace**, like `catalogentry` and `sbomrecord`,
  because a buyer and a seller are in different tenants and the event linking
  them belongs to neither. Reads are scoped by an ownership edge
  (`principal.Owners []orgID`), never by namespace.
- **Agents are not principals.** An agent carries `PrincipalId`; a spawned
  child inherits its parent's unless the Lux lineage attestation says otherwise.
  Resolution walks lineage to the first agent with a binding. An agent with no
  binding cannot be paid (fail closed; see `canPay`).
- **PII does not ride on the row.** The TIN, DOB and document scans go into a
  vault record encrypted with `cek.DeriveKey(master, "system", "principal-pii")`
  (the same scheme as the tenant stores) or into KMS. The principal row carries
  `TinRef`, `TinLast4`, `TinType`, `TinVerifiedAt`. The two existing plaintext
  `TaxId` fields get migrated out, not reused.

```
EconomicPrincipal
  Kind            individual | business | disregarded_entity
  LegalName, Country, TaxResidencies[]
  TinRef, TinLast4, TinType (SSN|EIN|ITIN|VAT|ABN|BN|…), TinVerifiedAt
  TaxForm         { Type: W9|W8BEN|W8BENE|…, SignedAt, ExpiresAt, DocRef }
  Verification    { KYCLevel, KYBStatus, Provider, CheckedAt }
  Screening       { Lists[], LastScreenedAt, Hit bool }
  PayoutMethods[] (bank refs, wallet addresses — refs only)
  Owners[]        IAM org ids that may act as it
```

## Primitive 2 — `EconomicEvent`

One row per completed economic transfer, whatever the rail.

```
Id              sha256("econ-event\0" + rail + ":" + settlementRef)
BuyerPrincipal, SellerPrincipal, AgentId, AgentLineage
ServiceCategory (enum, drives classification — not free text)
Gross, Fee, Tax, Withholding   int64 minor units + Currency
FMV             { AmountUSDMinor, Source, AsOf }   (crypto only)
Jurisdictions   derived at write time, frozen on the row
Rail            ach|wire|card|usdc|btc|x402|…
Proof           tx hash / processor charge id / attestation
OccurredAt
```

Three decisions that are not optional:

1. **The id is a function of the settlement, not a counter.** Exactly the
   `depositledger.creditKey` argument: every writer (webhook, watcher, x402
   receipt, retry) that sees one settlement lands on one row, with no
   coordination. `datastore.RunInTransaction` is a no-op, so this is the only
   dedupe that works here.
2. **Jurisdiction and classification are frozen at write time.** A report for
   tax year N must reproduce from the rows alone after the rules table changes.
3. **FMV is recorded, never used to credit.** CLAUDE.md is explicit that
   commerce has no price oracle and must not grow one to value a deposit. FMV
   for REPORTING is a different obligation and needs a source; it is stored with
   its `Source` and `AsOf` and never feeds a balance. Keep those two separate or
   one of them will eventually decide the other.

## `canPay` — one function, used twice

```
canPay(buyerAgent, sellerAgent, amount, category) -> Decision
Decision { allowed, requiredBeforePayment[], reportingObligations[],
           settlementMethods[], withholding, reasons[] }
```

- **Pure.** Inputs: both principals' compliance state (already loaded), the
  amount, the category, and a versioned rules table. No I/O. Split exactly like
  `depositwatch` (policy) / `depositledger` (I/O), so every rule is
  unit-testable against fixtures.
- **The settlement path calls the same function.** An advisory endpoint that
  says "allowed" while the settle path enforces something else is the
  `ListPlans`-vs-`lookupPlan` split already open in CLAUDE.md. Settlement
  refuses on `allowed == false` and writes `withholding` onto the event.
- **Fails closed.** Unbound agent, unscreened principal, expired W-8, or a
  jurisdiction with no row → `allowed: false` with the reason, never a default
  allow.
- **Rules are data rows, dated by tax year, each with a citation.**
  Thresholds (1099-NEC/-K, DAC7's seller de-minimis, UK/CA/AU platform rules,
  backup withholding, treaty rates) change by statute. Put them in rows like
  `models/rate`, not in code, and do not ship a threshold without the source
  it came from. The numbers in this doc are left out on purpose.

## A regulatory point the positioning glosses

"Hanzo need not be custodian" is true for money transmission and false for
reporting. DAC7, the UK/CA/AU OECD model rules and US 1099-K attach to the
**platform operator / payment settlement entity**, whether or not it touches the
funds. If Hanzo runs the marketplace and `canPay` gates the payment, Hanzo is
very likely the reporting party for those rails, crypto included (CARF / 1099-DA
next). The design assumes that rather than hoping otherwise, which is why the
event ledger is the product and the rails are plug-ins.

Separately: commerce already holds prepaid balances and MPC custody addresses.
Those are custodial today. The noncustodial story applies to the Lux x402 path,
not to the rails this repo operates now.

## Surface

Your sketch, mapped onto this repo's conventions (bare `/v1/<kind>`, system-ns
data behind a gate, no collisions with `/v1/billing/*`):

```
POST /v1/principal                         create (org-admin; owner edge = caller's org)
GET  /v1/principal/:id/compliance          Decision inputs + what is missing
POST /v1/principal/:id/taxform             W-9/W-8 submit → vault, returns ref
POST /v1/agent                             bind agent → principal (Lux attestation required)
POST /v1/agent/:id/spawn                   child inherits binding unless attested otherwise
GET  /v1/agent/:id/principal-attestation   signed statement: agent → principal, KYC level
POST /v1/commerce/canpay                   the pure Decision
POST /v1/job                               contract between two principals
POST /v1/settlement                        rail-agnostic; writes EconomicEvent
GET  /v1/report/:jurisdiction/:year        SuperAdmin; rebuilt from frozen events
```

## First slice (smallest thing that is real)

1. `models/economicprincipal`, `models/economicevent` — two kinds, registered
   in `util/hashid/kind.go` (next free: 292, 293). System namespace.
2. `agentcommerce/policy` — pure `CanPay` + a rules-row type, with a fixture
   table for one jurisdiction (US) and table tests. No thresholds shipped
   without a citation.
3. Wire `payment/x402` to its first caller: the paywall middleware's receipt
   (remote facilitator only — local settle cannot succeed) writes an
   `EconomicEvent` keyed `x402:<txHash>`, after calling `CanPay`.
4. PII vault stub that refuses to store a TIN unless the at-rest key is
   configured. No plaintext fallback.

Deliberately NOT in slice 1: KYC/KYB vendor, sanctions vendor, form generation,
fiat payouts, reputation. Each is an adapter behind an interface once the
principal + event exist. None of them is useful before that.
