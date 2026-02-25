/**
 * Commerce API client for Hanzo billing operations.
 *
 * Pure TypeScript -- no framework dependencies.
 * Default base URL: https://api.commerce.hanzo.ai
 */

import type {
  Balance,
  CommerceClientConfig,
  CommerceCreditBalance,
  CommerceCreditGrant,
  CommerceDiscountCode,
  CommerceInvoice,
  CommerceMeter,
  CommerceMeterEventsSummary,
  CommercePaymentMethod,
  CommercePortalOverview,
  CommercePlan,
  CommerceSpendAlert,
  CommerceSubscription,
  CommerceTransaction,
  CommerceUsageSummary,
} from './types'

import { DEFAULT_COMMERCE_URL } from './config'

const DEFAULT_TIMEOUT_MS = 10_000

// ── Error ───────────────────────────────────────────────────────────────────

export class CommerceApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'CommerceApiError'
    this.status = status
  }
}

// ── Client ──────────────────────────────────────────────────────────────────

export class Commerce {
  private readonly baseUrl: string
  private token: string | undefined

  constructor(config?: Partial<CommerceClientConfig>) {
    this.baseUrl = (config?.baseUrl ?? DEFAULT_COMMERCE_URL).replace(/\/+$/, '')
    this.token = config?.token
  }

  setToken(token: string): void {
    this.token = token
  }

  private async request<T>(
    path: string,
    opts?: {
      method?: string
      body?: unknown
      token?: string
      params?: Record<string, string>
    },
  ): Promise<T> {
    const url = new URL(path, this.baseUrl)
    if (opts?.params) {
      for (const [k, v] of Object.entries(opts.params)) {
        url.searchParams.set(k, v)
      }
    }

    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS)

    const headers: Record<string, string> = { Accept: 'application/json' }
    const authToken = opts?.token ?? this.token
    if (authToken) headers.Authorization = `Bearer ${authToken}`
    if (opts?.body) headers['Content-Type'] = 'application/json'

    try {
      const res = await fetch(url.toString(), {
        method: opts?.method ?? 'GET',
        headers,
        body: opts?.body ? JSON.stringify(opts.body) : undefined,
        signal: controller.signal,
      })

      if (!res.ok) {
        const text = await res.text().catch(() => '')
        throw new CommerceApiError(res.status, `${res.statusText}: ${text}`.trim())
      }

      return (await res.json()) as T
    } finally {
      clearTimeout(timer)
    }
  }

  // ── Balance ──────────────────────────────────────────────────────────────

  async getBalance(user: string, currency = 'usd', token?: string): Promise<Balance> {
    return this.request<Balance>('/api/v1/billing/balance', {
      params: { user, currency },
      token,
    })
  }

  async addDeposit(
    params: { user: string; currency?: string; amount: number; notes?: string; tags?: string[]; expiresIn?: string },
    token?: string,
  ): Promise<CommerceTransaction> {
    return this.request<CommerceTransaction>('/api/v1/billing/deposit', {
      method: 'POST',
      body: params,
      token,
    })
  }

  async grantStarterCredit(user: string, token?: string): Promise<CommerceTransaction> {
    return this.request<CommerceTransaction>('/api/v1/billing/credit', {
      method: 'POST',
      body: { user },
      token,
    })
  }

  // ── Transactions ─────────────────────────────────────────────────────────

  async getTransactions(
    user: string,
    params?: { limit?: number; offset?: number; currency?: string },
    token?: string,
  ): Promise<CommerceTransaction[]> {
    return this.request<CommerceTransaction[]>('/api/v1/billing/transactions', {
      params: {
        user,
        ...(params?.limit ? { limit: String(params.limit) } : {}),
        ...(params?.offset ? { offset: String(params.offset) } : {}),
        ...(params?.currency ? { currency: params.currency } : {}),
      },
      token,
    })
  }

  // ── Subscriptions ────────────────────────────────────────────────────────

  async subscribe(
    params: { planId: string; userId: string; paymentToken?: string },
    token?: string,
  ): Promise<CommerceSubscription> {
    return this.request<CommerceSubscription>('/api/v1/subscribe', {
      method: 'POST',
      body: params,
      token,
    })
  }

  async getSubscription(subscriptionId: string, token?: string): Promise<CommerceSubscription | null> {
    try {
      return await this.request<CommerceSubscription>(`/api/v1/subscribe/${subscriptionId}`, { token })
    } catch {
      return null
    }
  }

  async getUserSubscriptions(userId: string, token?: string): Promise<CommerceSubscription[]> {
    try {
      return await this.request<CommerceSubscription[]>('/api/v1/subscribe', {
        params: { userId },
        token,
      })
    } catch {
      return []
    }
  }

  async cancelSubscription(subscriptionId: string, token?: string): Promise<void> {
    await this.request<void>(`/api/v1/subscribe/${subscriptionId}`, {
      method: 'DELETE',
      token,
    })
  }

  async updateSubscription(
    subscriptionId: string,
    params: { planId: string },
    token?: string,
  ): Promise<CommerceSubscription> {
    return this.request<CommerceSubscription>(`/api/v1/subscribe/${subscriptionId}`, {
      method: 'PATCH',
      body: params,
      token,
    })
  }

  async applyDiscount(
    subscriptionId: string,
    code: string,
    token?: string,
  ): Promise<CommerceDiscountCode> {
    return this.request<CommerceDiscountCode>(`/api/v1/subscribe/${subscriptionId}/promotion`, {
      method: 'POST',
      body: { code },
      token,
    })
  }

  // ── Plans ────────────────────────────────────────────────────────────────

  async getPlans(token?: string): Promise<CommercePlan[]> {
    return this.request<CommercePlan[]>('/api/v1/plan', { token })
  }

  async getPlan(planId: string, token?: string): Promise<CommercePlan | null> {
    try {
      return await this.request<CommercePlan>(`/api/v1/plan/${planId}`, { token })
    } catch {
      return null
    }
  }

  // ── Invoices ─────────────────────────────────────────────────────────────

  async getInvoices(
    userId: string,
    params?: { limit?: number; offset?: number },
    token?: string,
  ): Promise<CommerceInvoice[]> {
    try {
      return await this.request<CommerceInvoice[]>('/api/v1/billing/invoices', {
        params: {
          user: userId,
          ...(params?.limit ? { limit: String(params.limit) } : {}),
          ...(params?.offset ? { offset: String(params.offset) } : {}),
        },
        token,
      })
    } catch {
      return []
    }
  }

  // ── Payment Methods ──────────────────────────────────────────────────────

  async getPaymentMethods(userId: string, token?: string): Promise<CommercePaymentMethod[]> {
    try {
      return await this.request<CommercePaymentMethod[]>('/api/v1/billing/payment-methods', {
        params: { user: userId },
        token,
      })
    } catch {
      return []
    }
  }

  async addPaymentMethod(
    params: CommercePaymentMethod,
    token?: string,
  ): Promise<CommercePaymentMethod> {
    return this.request<CommercePaymentMethod>('/api/v1/billing/payment-methods', {
      method: 'POST',
      body: params,
      token,
    })
  }

  async removePaymentMethod(methodId: string, token?: string): Promise<void> {
    await this.request<void>(`/api/v1/billing/payment-methods/${methodId}`, {
      method: 'DELETE',
      token,
    })
  }

  async setDefaultPaymentMethod(methodId: string, token?: string): Promise<void> {
    await this.request<void>(`/api/v1/billing/payment-methods/${methodId}/default`, {
      method: 'POST',
      token,
    })
  }

  // ── Usage ────────────────────────────────────────────────────────────────

  async getUsage(userId: string, token?: string): Promise<CommerceUsageSummary> {
    try {
      return await this.request<CommerceUsageSummary>('/api/v1/billing/usage', {
        params: { user: userId },
        token,
      })
    } catch {
      return { totalCost: 0, currency: 'usd', period: {}, records: [] }
    }
  }

  // ── Spend Alerts ─────────────────────────────────────────────────────────

  async getSpendAlerts(userId: string, token?: string): Promise<CommerceSpendAlert[]> {
    try {
      return await this.request<CommerceSpendAlert[]>('/api/v1/billing/spend-alerts', {
        params: { user: userId },
        token,
      })
    } catch {
      return []
    }
  }

  async createSpendAlert(
    params: { userId: string; title: string; threshold: number; currency?: string },
    token?: string,
  ): Promise<CommerceSpendAlert> {
    return this.request<CommerceSpendAlert>('/api/v1/billing/spend-alerts', {
      method: 'POST',
      body: params,
      token,
    })
  }

  async updateSpendAlert(
    alertId: string,
    params: { title?: string; threshold?: number },
    token?: string,
  ): Promise<CommerceSpendAlert> {
    return this.request<CommerceSpendAlert>(`/api/v1/billing/spend-alerts/${alertId}`, {
      method: 'PATCH',
      body: params,
      token,
    })
  }

  async deleteSpendAlert(alertId: string, token?: string): Promise<void> {
    await this.request<void>(`/api/v1/billing/spend-alerts/${alertId}`, {
      method: 'DELETE',
      token,
    })
  }

  // ── Discount Codes ───────────────────────────────────────────────────────

  async validateDiscountCode(code: string, token?: string): Promise<CommerceDiscountCode> {
    return this.request<CommerceDiscountCode>('/api/v1/billing/discount/validate', {
      params: { code },
      token,
    })
  }

  // ── Credit Grants ────────────────────────────────────────────────────────

  async getCreditGrants(userId: string, token?: string): Promise<CommerceCreditGrant[]> {
    try {
      return await this.request<CommerceCreditGrant[]>('/api/v1/billing/credit-grants', {
        params: { userId },
        token,
      })
    } catch {
      return []
    }
  }

  async getCreditBalance(userId: string, token?: string): Promise<CommerceCreditBalance> {
    try {
      return await this.request<CommerceCreditBalance>('/api/v1/billing/credit-balance', {
        params: { userId },
        token,
      })
    } catch {
      return { userId, balances: [] }
    }
  }

  // ── Portal ───────────────────────────────────────────────────────────────

  async getPortalOverview(customerId: string, token?: string): Promise<CommercePortalOverview> {
    return this.request<CommercePortalOverview>('/api/v1/billing/portal/overview', {
      params: { customerId },
      token,
    })
  }

  // ── Meters ───────────────────────────────────────────────────────────────

  async getMeters(token?: string): Promise<CommerceMeter[]> {
    try {
      return await this.request<CommerceMeter[]>('/api/v1/billing/meters', { token })
    } catch {
      return []
    }
  }

  async getMeterEventsSummary(userId: string, token?: string): Promise<CommerceMeterEventsSummary> {
    try {
      return await this.request<CommerceMeterEventsSummary>('/api/v1/billing/meter-events/summary', {
        params: { userId },
        token,
      })
    } catch {
      return { userId, meters: [], period: {} }
    }
  }
}
