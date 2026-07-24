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
}

export default Commerce
