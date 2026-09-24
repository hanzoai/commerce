# commerce.zap — ZAP schema for the Commerce subsystem.
#
# HIP-0106 unified-binary contract: every Hanzo subsystem ships its
# inter-subsystem call surface as a .zap schema, so the cloud co-resident and
# split-deploy paths share one source of truth.
#
# Commerce is a LIGHT ROUTER, NOT in PCI-DSS scope. PAN never crosses this
# boundary; the Payments + Vault services do.
#
# Code generation (Go client + server contract):
#
#   zapgen -out ./schema/gen schema/commerce.zap
#
# zapgen is the compiler for this dialect — hanzo/zap/go/cmd/zapgen (build it
# with `GOWORK=off go build ./cmd/zapgen`). Per interface it emits a typed
# client, a Handler contract and a Dispatch<Name> ordinal router over the
# github.com/zap-proto/go rpc runtime.
#
# NOT zapc: that is the Cap'n-Proto-lineage RUST codegen PLUGIN (zap/rust/zapc)
# that reads a code_generator_request on stdin. It does not read this dialect
# and has no `generate --lang` verb.
#
# Dialect, as enforced by the zapgen parser:
#   - `#` comments; a `package` declaration is REQUIRED and comes first
#   - top-level decls are `struct`, `interface`, `type` — there is no `enum`
#   - primitives are lowercase: bool u8 u16 u32 u64 i8 i16 i32 i64 f32 f64
#     text bytes list<T> bytes_fixed[N]
#   - a field is `Name type`; a braceless header plus an indented body is sugar
#     the desugar pass rewrites to the canonical brace form, auto-assigning each
#     field's `@byteOffset` from the running slot width
#   - an interface method is `name(param: Type) returns (out: Type)`; method
#     ordinals are 1-based in declaration order and are WIRE IDENTITY — APPEND
#     new methods, never reorder or remove
#
# Monetary amounts are cents (i64). Currency codes are lowercase ISO 4217.

package commerce

# ── Health ───────────────────────────────────────────────────────────────

struct HealthRequest
  Unused u8

struct HealthResponse
  Status  text
  Version text

# ── Tenant ───────────────────────────────────────────────────────────────

struct TenantRequest
  OrgId text

struct TenantConfig
  OrgId          text
  Brand          text
  EnabledMethods list<text>

# ── Billing ──────────────────────────────────────────────────────────────
#
# The money surface. Every struct below mirrors the JSON the live dispatcher
# (api/billing/zap.go) already accepts and returns: this schema describes what
# commerce serves TODAY, not an aspiration.
#
# A transaction type is `text`, not an enum — the dialect has none. Values are
# exactly transaction.Type: hold | holdRemoved | transfer | deposit | withdraw.

struct BalanceRequest
  User     text
  Currency text

struct Balance
  User      text
  Currency  text
  Balance   i64
  Holds     i64
  Available i64

struct BalanceAllRequest
  User text

struct BalanceEntry
  Currency  text
  Balance   i64
  Holds     i64
  Available i64

struct BalanceAllResponse
  User     text
  Balances list<BalanceEntry>

struct UsageQuery
  User     text
  Currency text

struct UsageItem
  TransactionId text
  Amount        i64
  Currency      text
  Notes         text
  CreatedAt     text

struct UsageList
  User  text
  Count i32
  Usage list<UsageItem>

struct UsageRecord
  User             text
  Currency         text
  Amount           i64
  Model            text
  Provider         text
  PromptTokens     i32
  CompletionTokens i32
  TotalTokens      i32
  RequestId        text
  Premium          bool
  Stream           bool
  Status           text
  ClientIp         text

struct UsageResult
  TransactionId text
  User          text
  Amount        i64
  Currency      text
  Type          text

struct DepositRequest
  User      text
  Currency  text
  Amount    i64
  Notes     text
  Tags      text
  ExpiresIn i32

struct TransactionResult
  TransactionId text
  User          text
  Amount        i64
  Currency      text
  Type          text

# ── Service interfaces ───────────────────────────────────────────────────

interface Health {
    check(req: HealthRequest) returns (resp: HealthResponse)
}

interface Tenant {
    get(req: TenantRequest) returns (resp: TenantConfig)
}

# Billing mirrors the five methods ZapDispatch actually routes
# (api/billing/zap.go) — no more. Reads and recordUsage (a withdraw/debit) are
# admin-scoped. deposit MINTS spendable balance and so carries the platform-only
# bar: the internal service token or a platform SuperAdmin ONLY, never an
# org-level Admin bit. That gate is middleware.PlatformOnly on the REST route and
# zapMintMethods in the dispatcher — it is a property of the CALLER, not of this
# schema, so generating a client from this file confers no authority to deposit.
interface Billing {
    getBalance(req: BalanceRequest) returns (resp: Balance)
    getBalanceAll(req: BalanceAllRequest) returns (resp: BalanceAllResponse)
    getUsage(req: UsageQuery) returns (resp: UsageList)
    recordUsage(req: UsageRecord) returns (resp: UsageResult)
    deposit(req: DepositRequest) returns (resp: TransactionResult)
}
