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
}

export default Commerce
