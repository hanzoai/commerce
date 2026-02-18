# Hanzo Billing Service - ZAP Schema
# Single source of truth for all balance/usage/transaction operations.
#
# Server:  Commerce (Go/Gin) at commerce.hanzo.ai
# Clients: Cloud-API (Go), LLM Gateway (Python), Console (TypeScript), hanzo/node (Rust)
#
# All monetary amounts are in cents (Int64).
# Currency codes are lowercase ISO 4217 (e.g. "usd", "eur", "jpy").
#
# Code generation:
#   zapc generate billing.zap --lang go     --out ./gen/go/
#   zapc generate billing.zap --lang python  --out ./gen/python/
#   zapc generate billing.zap --lang ts      --out ./gen/ts/
#   zapc generate billing.zap --lang rust    --out ./gen/rust/

# =============================================================================
# Transaction Types
# =============================================================================

enum TransactionType
  hold
  holdRemoved
  transfer
  deposit
  withdraw

# =============================================================================
# Balance
# =============================================================================

struct Balance
  user Text
  currency Text
  balance Int64       # total balance in cents
  holds Int64         # held amount in cents
  available Int64     # balance - holds, clamped >= 0

struct BalanceEntry
  currency Text
  balance Int64
  holds Int64
  available Int64

struct BalanceAllResponse
  user Text
  balances List(BalanceEntry)

# =============================================================================
# Usage
# =============================================================================

struct UsageRecord
  user Text
  currency Text
  amount Int64        # cost in cents
  model Text
  provider Text
  promptTokens Int32
  completionTokens Int32
  totalTokens Int32
  requestId Text
  premium Bool
  stream Bool
  status Text
  clientIp Text

struct UsageResult
  transactionId Text
  user Text
  amount Int64
  currency Text
  type TransactionType

struct UsageQuery
  user Text
  currency Text

struct UsageItem
  transactionId Text
  amount Int64
  currency Text
  notes Text
  metadata Data
  createdAt Text

struct UsageList
  user Text
  count Int32
  usage List(UsageItem)

# =============================================================================
# Deposit
# =============================================================================

struct DepositRequest
  user Text
  currency Text
  amount Int64
  notes Text
  tags Text
  metadata Data

struct TransactionResult
  transactionId Text
  user Text
  amount Int64
  currency Text
  type TransactionType

# =============================================================================
# Service Interface
# =============================================================================

interface BillingService
  # GET /api/v1/billing/balance?user=&currency=
  getBalance (user Text, currency Text) -> (balance Balance)

  # GET /api/v1/billing/balance/all?user=
  getBalanceAll (user Text) -> (response BalanceAllResponse)

  # GET /api/v1/billing/usage?user=&currency=
  getUsage (query UsageQuery) -> (list UsageList)

  # POST /api/v1/billing/usage
  recordUsage (record UsageRecord) -> (result UsageResult)

  # POST /api/v1/billing/deposit (future)
  deposit (request DepositRequest) -> (result TransactionResult)
