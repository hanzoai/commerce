// Minimal same-origin billing client — replaces the never-published `@hanzo/commerce-client`.
// Talks to the REAL user-scoped billing routes (identity from the OIDC bearer + X-Org-Id):
//   GET /v1/billing/me/balance     -> GetMyBalance      { available, ... }
//   GET /v1/billing/credit-balance -> GetCreditBalance  { available }
//   GET /v1/billing/invoices       -> ListInvoices      (admin-scoped; empties gracefully)
// Every call degrades to null/[] on non-2xx so the Billing page renders a clean empty state.
const API_BASE = process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://commerce.hanzo.ai'

export interface CommerceConfig {
  token?: string | null
  org?: string | null
}

export interface Balance {
  available?: number
  [k: string]: unknown
}

export interface Invoice {
  id: string
  number?: string
  status?: string
  total?: number
  currency?: string
  createdAt?: string
  [k: string]: unknown
}

export interface Plan {
  slug: string
  name: string
  description?: string
  price: number
  priceAnnual?: number
  currency: string
  interval: string
  category?: string
  trialPeriodDays?: number
  contactSales?: boolean
  popular?: boolean
  features?: string[]
}

export interface Tier {
  tier?: {
    name?: string
    displayName?: string
    maxAgents?: number
    unlimitedAgents?: boolean
    dailyCreditsCents?: number
    allowedModels?: string[]
  }
  balance?: {
    currency?: string
    effectiveAvailable?: number
    creditsRemaining?: number
    prepaidAvailable?: number
    dailyRemaining?: number
  }
}

// ── Usage / analytics (per-org, per-subject) ─────────────────────────────────
// The subject is the IAM user id in `org/username` form (JWT `sub`), which the
// billing usage/tier routes take as their `user` query param. Every read
// degrades to a graceful empty value so the dashboard never crashes.

/** One api-usage withdrawal (an AI/API call charged to the wallet). */
export interface UsageItem {
  transactionId: string
  amount: number // integer cents (may be signed; charts take the magnitude)
  currency?: string
  notes?: string
  metadata?: Record<string, unknown>
  createdAt?: string
}

export interface UsageResponse {
  user: string
  count: number
  usage: UsageItem[]
}

/** Unified plan + included-allowance + consumed + overage + balance rollup. */
export interface UsageRollup {
  user: string
  plan: string
  currency: string
  period: string
  included: {
    monthlyCents: number
    grantedCents: number
    consumedCents: number
    remainingCents: number
  }
  consumedCents: number
  overageCents: number
  balance: {
    balanceCents: number
    holdsCents: number
    availableCents: number
  }
}

/** Aggregated value for one meter over a period. */
export interface MeterSummary {
  meterId: string
  meterName: string
  userId: string
  aggregationType: string
  value: number
  eventCount: number
  periodStart: string
  periodEnd: string
}

export interface PaymentConfig {
  provider: 'square'
  applicationId: string
  locationId: string
  environment: 'sandbox' | 'production'
}

export interface StoreAccess {
  allowed: boolean
  storeId?: string
  status: 'active' | 'trial' | 'payment_required' | 'store_required' | 'unavailable'
}

export interface Subscription {
  status?: 'trialing' | 'active' | 'past_due' | 'canceled' | 'unpaid' | string
  planSlug?: string
  trialEndsAt?: string
  currentPeriodEnd?: string
  [k: string]: unknown
}

export interface SubscribeCardInput {
  planSlug: string
  sourceId: string
  currency: string
}

// A saved card / payment method (GET /v1/billing/payment-methods).
export interface CardDetails {
  brand?: string
  last4?: string
  expMonth?: number
  expYear?: number
}

export interface PaymentMethod {
  id: string
  customerId?: string
  type?: string
  name?: string
  isDefault?: boolean
  card?: CardDetails
  providerRef?: string
  [k: string]: unknown
}

// One org subscription (GET /v1/billing/subscriptions).
export interface BillingSubscription {
  id: string
  userId?: string
  planId?: string
  status?: string
  quantity?: number
  currentPeriodStart?: string
  currentPeriodEnd?: string
  cancelAtPeriodEnd?: boolean
  providerType?: string
  trialEnd?: string
  canceledAt?: string
  plan?: { id?: string; name?: string; price?: number; currency?: string; interval?: string }
  [k: string]: unknown
}

// Auto-recharge config (GET/PUT /v1/billing/auto-recharge).
export interface AutoRecharge {
  enabled: boolean
  thresholdCents: number
  amountCents: number
  currency?: string
  lastRechargedAt?: string
}

// The bucketed balance (GET /v1/billing/me/balance) — credit vs prepaid split.
export interface BalanceDetail extends Balance {
  balance?: number
  holds?: number
  creditsGranted?: number
  creditsRemaining?: number
  prepaidBalance?: number
  prepaidAvailable?: number
  card?: { onFile?: boolean; brand?: string; last4?: string; isDefault?: boolean }
}

export class Commerce {
  private token?: string | null
  private org?: string | null

  constructor(config: CommerceConfig = {}) {
    this.token = config.token
    this.org = config.org
  }

  private headers(): HeadersInit {
    const h: HeadersInit = { 'Content-Type': 'application/json' }
    if (this.token) h['Authorization'] = `Bearer ${this.token}`
    if (this.org) h['X-Org-Id'] = this.org
    return h
  }

  private async get<T>(path: string, fallback: T): Promise<T> {
    try {
      const res = await fetch(`${API_BASE}${path}`, { headers: this.headers() })
      if (!res.ok) return fallback
      return (await res.json()) as T
    } catch {
      return fallback
    }
  }

  // The API renders errors as { error: { message, code } } (util/json/http Fail);
  // it also emits a flat { message } or a string { error } in places. Surface a
  // real reason so callers can toast something useful, not a bare status code.
  private async fail(res: Response): Promise<never> {
    let message = `Request failed: ${res.status}`
    try {
      const text = await res.text()
      if (text) {
        try {
          const body = JSON.parse(text) as { message?: string; error?: string | { message?: string } }
          const nested = typeof body.error === 'object' ? body.error?.message : body.error
          message = body.message || nested || text
        } catch {
          message = text
        }
      }
    } catch {
      // keep the status-code fallback
    }
    throw new Error(message)
  }

  private async send<T>(method: string, path: string, body?: unknown, extra: Record<string, string> = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method,
      headers: { ...this.headers(), ...extra },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    if (!res.ok) return this.fail(res)
    // Some mutations (or empty 200s) carry no JSON body — tolerate that.
    const text = await res.text()
    return (text ? JSON.parse(text) : {}) as T
  }

  private async post<T>(path: string, body: unknown, extra: Record<string, string> = {}): Promise<T> {
    return this.send<T>('POST', path, body, extra)
  }

  private async patch<T>(path: string, body: unknown): Promise<T> {
    return this.send<T>('PATCH', path, body)
  }

  private async put<T>(path: string, body: unknown): Promise<T> {
    return this.send<T>('PUT', path, body)
  }

  private async del<T>(path: string): Promise<T> {
    return this.send<T>('DELETE', path)
  }

  async getBalance(_subject = 'me'): Promise<Balance | null> {
    return this.get<Balance | null>('/v1/billing/me/balance', null)
  }

  async getCreditBalance(_subject = 'me'): Promise<Balance | null> {
    return this.get<Balance | null>('/v1/billing/credit-balance', null)
  }

  async getInvoices(_subject = 'me', opts: { limit?: number } = {}): Promise<Invoice[]> {
    const qs = opts.limit ? `?display=${opts.limit}` : ''
    // ListInvoices returns { invoices, count }; tolerate a bare array or the
    // legacy { models } envelope too.
    const body = await this.get<{ invoices?: Invoice[]; models?: Invoice[] } | Invoice[] | null>(
      `/v1/billing/invoices${qs}`,
      null,
    )
    if (!body) return []
    if (Array.isArray(body)) return body
    if (Array.isArray(body.invoices)) return body.invoices
    return Array.isArray(body.models) ? body.models : []
  }

  // Stream an invoice PDF (GET /v1/billing/invoices/:id/pdf) with the bearer
  // header and trigger a browser download. A plain <a href> can't carry the
  // Authorization header, so the fetch-to-blob path is the ONE way to download.
  async downloadInvoicePdf(id: string, filename?: string): Promise<void> {
    const res = await fetch(`${API_BASE}/v1/billing/invoices/${encodeURIComponent(id)}/pdf`, {
      headers: this.headers(),
    })
    if (!res.ok) return this.fail(res)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    try {
      const a = document.createElement('a')
      a.href = url
      a.download = filename || `invoice-${id}.pdf`
      document.body.appendChild(a)
      a.click()
      a.remove()
    } finally {
      URL.revokeObjectURL(url)
    }
  }

  async payInvoice(id: string): Promise<{ invoice?: Invoice; collection?: { success?: boolean } }> {
    return this.post(`/v1/billing/invoices/${encodeURIComponent(id)}/pay`, {})
  }

  async voidInvoice(id: string): Promise<Invoice> {
    return this.post(`/v1/billing/invoices/${encodeURIComponent(id)}/void`, {})
  }

  // ── Subscriptions ──────────────────────────────────────────────────────────
  async listSubscriptions(): Promise<BillingSubscription[]> {
    const body = await this.get<{ subscriptions?: BillingSubscription[] } | BillingSubscription[] | null>(
      '/v1/billing/subscriptions',
      null,
    )
    if (!body) return []
    if (Array.isArray(body)) return body
    return Array.isArray(body.subscriptions) ? body.subscriptions : []
  }

  async cancelSubscription(id: string, atPeriodEnd = true): Promise<BillingSubscription> {
    return this.post(`/v1/billing/subscriptions/${encodeURIComponent(id)}/cancel`, { atPeriodEnd })
  }

  async reactivateSubscription(id: string): Promise<BillingSubscription> {
    return this.post(`/v1/billing/subscriptions/${encodeURIComponent(id)}/reactivate`, {})
  }

  // Change/upgrade/downgrade an existing subscription's plan (PATCH). `planId`
  // is a catalog slug from GET /v1/billing/plans — never hardcoded.
  async changePlan(id: string, planId: string, prorate = true): Promise<BillingSubscription> {
    return this.patch(`/v1/billing/subscriptions/${encodeURIComponent(id)}`, { planId, prorate })
  }

  // ── Payment methods ────────────────────────────────────────────────────────
  async listPaymentMethods(customerId: string): Promise<PaymentMethod[]> {
    const body = await this.get<PaymentMethod[] | null>(
      `/v1/billing/payment-methods?customerId=${encodeURIComponent(customerId)}`,
      null,
    )
    return Array.isArray(body) ? body : []
  }

  // Vault a tokenized Square card as a reusable card-on-file. `sourceId` is the
  // Web Payments SDK nonce; `providerRef` carries it so the backend vaults it.
  async addCard(customerId: string, sourceId: string): Promise<PaymentMethod> {
    return this.post('/v1/billing/payment-methods', {
      customerId,
      type: 'card',
      providerRef: sourceId,
      providerType: 'square',
    })
  }

  async removePaymentMethod(id: string): Promise<{ deleted?: boolean; id?: string }> {
    return this.del(`/v1/billing/payment-methods/${encodeURIComponent(id)}`)
  }

  async setDefaultPaymentMethod(customerId: string, paymentMethodId: string): Promise<PaymentMethod> {
    return this.post(`/v1/billing/customers/${encodeURIComponent(customerId)}/default-payment-method`, {
      paymentMethodId,
    })
  }

  // ── Top-up + auto-recharge ─────────────────────────────────────────────────
  // Top up spendable credit by charging a SAVED card (POST /topup).
  async topup(userId: string, paymentMethodId: string, amountCents: number, currency = 'usd') {
    return this.post<{ transactionId: string; balanceCents: number; status: string }>('/v1/billing/topup', {
      userId,
      paymentMethodId,
      amountCents,
      currency,
    })
  }

  // Top up with a freshly-tokenized Square nonce (no saved card required).
  async topupWithToken(sourceId: string, amountCents: number, currency = 'usd', idempotencyKey = crypto.randomUUID()) {
    return this.post<{ transactionId: string; balanceCents: number; status: string }>(
      '/v1/billing/topup/token',
      { sourceId, amountCents, currency },
      { 'X-Idempotency-Key': idempotencyKey },
    )
  }

  async getAutoRecharge(): Promise<AutoRecharge | null> {
    return this.get<AutoRecharge | null>('/v1/billing/auto-recharge', null)
  }

  async setAutoRecharge(input: {
    enabled: boolean
    thresholdCents: number
    amountCents: number
    currency?: string
  }): Promise<AutoRecharge> {
    return this.put('/v1/billing/auto-recharge', { currency: 'usd', ...input })
  }

  async getPlans(): Promise<Plan[]> {
    return this.get<Plan[]>('/v1/billing/plans', [])
  }

  async getTier(subject: string): Promise<Tier | null> {
    return this.get<Tier | null>(`/v1/billing/tier?user=${encodeURIComponent(subject)}`, null)
  }

  // Per-subject api-usage ledger (AI/API charges). Empties gracefully.
  async getUsage(subject: string): Promise<UsageResponse> {
    return this.get<UsageResponse>(`/v1/billing/usage?user=${encodeURIComponent(subject)}`, {
      user: subject,
      count: 0,
      usage: [],
    })
  }

  // Plan + included allowance + consumed + overage + balance for the month.
  async getUsageRollup(subject: string, plan?: string): Promise<UsageRollup | null> {
    const qs = plan ? `&plan=${encodeURIComponent(plan)}` : ''
    return this.get<UsageRollup | null>(
      `/v1/billing/usage-rollup?user=${encodeURIComponent(subject)}${qs}`,
      null,
    )
  }

  // Aggregated metered usage for one meter (requires a meterId). Null on any miss.
  async getMeterSummary(params: {
    meterId: string
    userId?: string
    periodStart?: string
    periodEnd?: string
  }): Promise<MeterSummary | null> {
    const qs = new URLSearchParams({ meterId: params.meterId })
    if (params.userId) qs.set('userId', params.userId)
    if (params.periodStart) qs.set('periodStart', params.periodStart)
    if (params.periodEnd) qs.set('periodEnd', params.periodEnd)
    return this.get<MeterSummary | null>(`/v1/billing/meter-events/summary?${qs.toString()}`, null)
  }

  async getPaymentConfig(): Promise<PaymentConfig | null> {
    return this.get<PaymentConfig | null>('/v1/billing/payment-config', null)
  }

  async getStoreAccess(storeId?: string): Promise<StoreAccess | null> {
    try {
      const h = this.headers() as Record<string, string>
      if (storeId) h['X-Store-Id'] = storeId
      const res = await fetch(`${API_BASE}/v1/store/access`, { headers: h })
      if (!res.ok) return null
      return (await res.json()) as StoreAccess
    } catch {
      return null
    }
  }

  async startStoreTrial(storeId: string) {
    return this.post<{ started: boolean; reason?: string; storeId?: string }>(
      `/v1/store/${encodeURIComponent(storeId)}/trial`,
      {},
    )
  }

  async subscribe(sourceId: string, storeId: string, planId = 'pro', idempotencyKey = crypto.randomUUID()) {
    return this.post<{ subscriptionId: string; status: string }>('/v1/billing/subscribe/card', {
      sourceId,
      storeId,
      planId,
    }, { 'X-Idempotency-Key': idempotencyKey })
  }

  // ── Account paywall (Medusa-Cloud-style /subscribe) ────────────────────────
  // The account-level subscription (not the per-store one `subscribe()` mints):
  // tokenized Square card → the chosen plan. The backend subscribeCardRequest reads
  // { sourceId, planId, currency } — the selected plan SLUG is a valid planId
  // (resolveSubscriptionPlan matches it against the plan catalog) — so map the
  // client input to that contract here.
  //
  // Pass a STABLE idempotencyKey per checkout attempt so a double-submit replays
  // the first charge instead of vaulting a second card / minting a second sub.
  async subscribeCard(input: SubscribeCardInput, idempotencyKey = crypto.randomUUID()) {
    return this.post<{ subscriptionId: string; status: string }>(
      '/v1/billing/subscribe/card',
      { sourceId: input.sourceId, planId: input.planSlug, currency: input.currency },
      { 'X-Idempotency-Key': idempotencyKey },
    )
  }

  // The current account subscription — the paywall reads it to surface a
  // trialing / active state. Degrades to null so the paywall renders clean.
  async getSubscription(): Promise<Subscription | null> {
    return this.get<Subscription | null>('/v1/billing/subscription', null)
  }

  // Idempotent free-trial start (7-day). Backend is the idempotency authority.
  async startTrial() {
    return this.post<{ started: boolean; status?: string; trialEndsAt?: string }>('/v1/billing/trial', {})
  }

  // Redeem an invite code → org entitlement (shares the inviter's store access).
  async redeemInvite(code: string) {
    return this.post<{ redeemed: boolean; org?: string; status?: string }>(
      '/v1/commerce/invite/redeem',
      { code },
    )
  }
}

export default Commerce
