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
  currency: string
  interval: string
  trialPeriodDays?: number
  features?: string[]
}

export interface Tier {
  tier?: { name?: string; displayName?: string }
  balance?: { effectiveAvailable?: number; creditsRemaining?: number; prepaidAvailable?: number }
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
  // Collected by the paywall for display/validation only. The backend
  // subscribeCardRequest does not persist them, so they never hit the wire.
  legalName?: string
  billingEmail?: string
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

  private async post<T>(path: string, body: unknown, extra: Record<string, string> = {}): Promise<T> {
    const res = await fetch(`${API_BASE}${path}`, {
      method: 'POST',
      headers: { ...this.headers(), ...extra },
      body: JSON.stringify(body),
    })
    if (!res.ok) {
      const message = await res.text()
      throw new Error(message || `Request failed: ${res.status}`)
    }
    return (await res.json()) as T
  }

  async getBalance(_subject = 'me'): Promise<Balance | null> {
    return this.get<Balance | null>('/v1/billing/me/balance', null)
  }

  async getCreditBalance(_subject = 'me'): Promise<Balance | null> {
    return this.get<Balance | null>('/v1/billing/credit-balance', null)
  }

  async getInvoices(_subject = 'me', opts: { limit?: number } = {}): Promise<Invoice[]> {
    const qs = opts.limit ? `?display=${opts.limit}` : ''
    const body = await this.get<{ models?: Invoice[] } | Invoice[] | null>(`/v1/billing/invoices${qs}`, null)
    if (!body) return []
    if (Array.isArray(body)) return body
    return Array.isArray(body.models) ? body.models : []
  }

  async getPlans(): Promise<Plan[]> {
    return this.get<Plan[]>('/v1/billing/plans', [])
  }

  async getTier(subject: string): Promise<Tier | null> {
    return this.get<Tier | null>(`/v1/billing/tier?user=${encodeURIComponent(subject)}`, null)
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
  // tokenized Square card → the pro plan. The backend subscribeCardRequest reads
  // { sourceId, planId, currency } — the selected plan SLUG is a valid planId
  // (resolveSubscriptionPlan matches it against the plan catalog) — so map the
  // client input to that contract here. legalName/billingEmail are paywall-only
  // (not persisted server-side) and never hit the wire.
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
