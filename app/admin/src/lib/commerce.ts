/**
 * The ONE Hanzo Commerce & Billing client the admin talks to.
 *
 * Talks directly to `https://api.hanzo.ai` using the signed-in IAM bearer token
 * and scopes every request to the active organization via `X-Org-Id`.
 *
 * - Storefront/catalog nouns (`product`, `order`, `cart`, `store`, ...) resolve
 *   under `/v1/commerce/<kind>`.
 * - Money/billing endpoints (`/v1/billing/transactions`, `/v1/billing/ledger`,
 *   `/v1/billing/subscriptions`, `/v1/billing/invoices`, `/v1/billing/payouts`,
 *   `/v1/billing/credits`, `/v1/billing/usage`) pass an explicit `/v1/...` path.
 *
 * Failures are THROWN with their HTTP status so `@hanzo/ui/product`'s
 * `classifyBackend` renders the honest state (402 → add credits, 403 → not
 * enabled, 404 → not mounted here). Nothing degrades to a fabricated empty list.
 */
import type { Organization } from '@hanzo/iam'
import type { CommerceList } from '@hanzo/ui/product'

const API_BASE = process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://api.hanzo.ai'

/** Every merchant store resource hangs off the commerce plugin's one prefix. */
const V1 = '/v1/commerce'

/** A `/v1` failure carrying its status — the shape `classifyBackend` reads. */
export class CommerceError extends Error {
  readonly status: number
  constructor(message: string, status = 0) {
    super(message)
    this.name = 'CommerceError'
    this.status = status
  }
}

/** The bearer the dashboard syncs from `@hanzo/iam` — ONE token source. */
let accessToken: string | null = null
export const setAccessToken = (t: string | null) => {
  accessToken = t
}

function headers(org?: string | null): HeadersInit {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (accessToken) h.Authorization = `Bearer ${accessToken}`
  if (org) h['X-Org-Id'] = org
  return h
}

async function call<T>(path: string, org: string | null, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, { ...init, headers: headers(org) })
  if (!res.ok) {
    const body = await res.text()
    throw new CommerceError(body.slice(0, 300) || `${init?.method ?? 'GET'} ${path} → ${res.status}`, res.status)
  }
  return res.status === 204 ? (undefined as T) : ((await res.json()) as T)
}

/** Resolve a catalog kind (`product`, `order`, or `/v1/billing/transactions`) to its API endpoint path. */
export function endpointPath(kind: string): string {
  return kind.startsWith('/') ? kind : `${V1}/${kind}`
}

function listQuery(path: string, display: number): string {
  if (path === '/v1/billing/ledger') return `${path}?range=90d`
  if (path === '/v1/billing/transactions' || path === '/v1/billing/invoices' || path === '/v1/commerce/order') {
    return `${path}?limit=${display}`
  }
  if (path.startsWith('/v1/billing/')) return path
  return `${path}?page=1&display=${display}`
}

type RecordRow = Record<string, unknown>

function normalizeRow(path: string, raw: unknown): RecordRow {
  const r = (raw && typeof raw === 'object' ? { ...(raw as RecordRow) } : {}) as RecordRow
  if (path === '/v1/billing/subscriptions') {
    const plan = r.plan && typeof r.plan === 'object' ? (r.plan as RecordRow) : null
    if (!r.planName) r.planName = plan?.name ?? plan?.slug ?? r.planId ?? ''
    if (!r.currency) r.currency = plan?.currency ?? 'usd'
    if (r.priceCents == null && typeof plan?.price === 'number') r.priceCents = plan.price
  } else if (path === '/v1/commerce/order') {
    if (!r.number && r.id) r.number = r.id
    if (r.total == null && typeof r.totalCents === 'number') r.total = r.totalCents
  } else if (path === '/v1/commerce/cart') {
    if (r.itemCount == null && Array.isArray(r.items)) r.itemCount = r.items.length
  } else if (path === '/v1/billing/transactions' || path === '/v1/billing/ledger') {
    if (r.amount == null && typeof r.cents === 'number') r.amount = r.cents
    if (!r.createdAt && r.date) r.createdAt = r.date
    if (!r.type && r.account) r.type = r.account
    if (!r.description && r.notes) r.description = r.notes
  } else if (path === '/v1/billing/invoices') {
    if (!r.number && r.id) r.number = r.id
    if (r.amountCents == null) r.amountCents = r.totalCents ?? r.total ?? r.amount
  } else if (path === '/v1/billing/usage') {
    if (!r.description && r.name) r.description = r.name
    if (r.amountCents == null && typeof r.amount === 'number') r.amountCents = r.amount
  }
  return r
}

const ENVELOPE_KEYS = [
  'models',
  'orders',
  'carts',
  'transactions',
  'subscriptions',
  'invoices',
  'payouts',
  'grants',
  'usage',
  'items',
] as const

/** One page of any commerce or billing resource. Handles all canonical envelope shapes. */
export async function list<T>(kind: string, org: string | null, display = 50): Promise<CommerceList<T>> {
  const path = endpointPath(kind)
  const body = await call<Record<string, unknown> | unknown[]>(listQuery(path, display), org)
  if (Array.isArray(body)) {
    const rows = body.slice(0, display).map((r) => normalizeRow(path, r)) as T[]
    return { rows, count: body.length }
  }
  let rawRows: unknown[] = []
  for (const key of ENVELOPE_KEYS) {
    const val = body[key]
    if (Array.isArray(val)) {
      rawRows = val
      break
    }
  }
  const rows = rawRows.map((r) => normalizeRow(path, r)) as T[]
  const count = typeof body.count === 'number' ? body.count : rows.length
  return { rows, count }
}

export const create = <T>(kind: string, data: Partial<T>, org: string | null): Promise<T> =>
  call<T>(endpointPath(kind), org, { method: 'POST', body: JSON.stringify(data) })

export const remove = (kind: string, id: string, org: string | null): Promise<void> =>
  call<void>(`${endpointPath(kind)}/${id}`, org, { method: 'DELETE' })

/** The caller-org's active store, or `null` when it has none yet (a 404 is not a failure). */
export async function currentStore(org: string | null): Promise<Store | null> {
  try {
    const body = await call<{ store?: Store } & Store>(`${V1}/store/current`, org)
    const store = body?.store ?? body
    return store?.id ? store : null
  } catch (e) {
    if (e instanceof CommerceError && e.status === 404) return null
    throw e
  }
}

export type OrgBalance = {
  balance: number
  available: number
  holds: number
  cash?: number
  credit?: number
  due?: number
  account?: string
  currency?: string
}

/** The active org's spendable balance from `/v1/billing/balance`. */
export async function orgBalance(org: string | null): Promise<OrgBalance | null> {
  try {
    return await call<OrgBalance>('/v1/billing/balance', org)
  } catch (e) {
    if (e instanceof CommerceError && (e.status === 404 || e.status === 403)) return null
    throw e
  }
}

/** Organizations visible to the signed-in caller via Hanzo IAM (`/v1/iam/organizations`). */
export async function listIamOrganizations(): Promise<Organization[]> {
  const body = await call<{ organizations?: Organization[] } | Organization[]>('/v1/iam/organizations', null)
  if (Array.isArray(body)) return body
  return Array.isArray(body.organizations) ? body.organizations : []
}

export type Store = {
  id: string
  name: string
  slug: string
  domain?: string
  currency?: string
  createdAt?: string
}
