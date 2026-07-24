// Same-origin Hanzo Commerce API. The live backend serves the merchant resources at
// BARE `/v1/<kind>` (singular, no hyphens, NO `/api` and NO `/commerce` segment):
//   /v1/product · /v1/order · /v1/c/user · /v1/collection · /v1/stocklocation · /v1/store/current
// (`/api/v1/*` is the Next.js SPA catch-all and returns HTML — never the API.)
// List responses use the envelope { page, display, count, models[], facets } — `models`
// + `count` are what we consume. Org is resolved from the bearer token; X-Org-Id scopes it.
const API_BASE = process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://commerce.hanzo.ai'

export interface ListResponse<T> {
  count: number
  models: T[]
  page: number
  display: number
}

export interface ListParams {
  page?: number
  display?: number
  sort?: string
  q?: string
  [key: string]: string | number | undefined
}

// Auth context — token set from dashboard layout, org passed per-call from hooks
let _accessToken: string | null = null
let _storeId: string | null = null

export function setAccessToken(token: string | null) {
  _accessToken = token
}

export function setStoreId(storeId: string | null) {
  _storeId = storeId
}

function headers(org?: string | null): HeadersInit {
  const h: HeadersInit = { 'Content-Type': 'application/json' }
  if (_accessToken) h['Authorization'] = `Bearer ${_accessToken}`
  if (org) h['X-Org-Id'] = org
  if (_storeId) h['X-Store-Id'] = _storeId
  return h
}

function emptyList<T>(params?: ListParams): ListResponse<T> {
  return { count: 0, models: [], page: params?.page ?? 1, display: params?.display ?? 20 }
}

// List reads degrade to a graceful empty envelope on any non-2xx (unfunded-org 402,
// permission 403, not-found 404): the UI shows an empty table, never a crash/error toast.
export async function fetchList<T>(kind: string, params?: ListParams, org?: string | null): Promise<ListResponse<T>> {
  const url = new URL(`${API_BASE}/v1/${kind}`)
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined) url.searchParams.set(k, String(v))
    })
  }
  try {
    const res = await fetch(url.toString(), { headers: headers(org) })
    if (!res.ok) return emptyList<T>(params)
    const body = await res.json()
    // Tolerate the envelope, a bare array, or a legacy { <kind>s: [] } shape.
    if (Array.isArray(body)) return { count: body.length, models: body, page: params?.page ?? 1, display: params?.display ?? body.length }
    if (Array.isArray(body?.models)) return body as ListResponse<T>
    return emptyList<T>(params)
  } catch {
    return emptyList<T>(params)
  }
}

export async function fetchOne<T>(kind: string, id: string, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/v1/${kind}/${id}`, { headers: headers(org) })
  if (!res.ok) throw new Error(`Failed to fetch ${kind}/${id}: ${res.status}`)
  return res.json()
}

export async function createOne<T>(kind: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/v1/${kind}`, {
    method: 'POST',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Failed to create ${kind}: ${res.status}`)
  return res.json()
}

export async function updateOne<T>(kind: string, id: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/v1/${kind}/${id}`, {
    method: 'PATCH',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Failed to update ${kind}/${id}: ${res.status}`)
  return res.json()
}

export async function deleteOne(kind: string, id: string, org?: string | null): Promise<void> {
  const res = await fetch(`${API_BASE}/v1/${kind}/${id}`, {
    method: 'DELETE',
    headers: headers(org),
  })
  if (!res.ok) throw new Error(`Failed to delete ${kind}/${id}: ${res.status}`)
}

export async function fetchCount(kind: string, org?: string | null): Promise<number> {
  const res = await fetchList(kind, { page: 1, display: 1 }, org)
  return res.count
}

// ── Store (merchant surface) ─────────────────────────────────────────────────
// GET /v1/store/current is meter-EXEMPT and returns the caller-org's real store,
// including its listings, keyed by slug. Live today (no billing gate).
export interface StoreListing {
  slug: string
  sku?: string
  name: string
  description?: string
  currency?: string
  price?: number
  available?: boolean
  sold?: number | null
  headerImage?: { type?: string; alt?: string; url?: string }
}

export interface CurrentStore {
  id: string
  name: string
  slug: string
  domain?: string
  currency?: string
  prefix?: string
  address?: unknown
  listings?: Record<string, StoreListing>
  createdAt?: string
  updatedAt?: string
  // Legacy display fields some admin views still reference (absent from
  // the live /v1/store/current response — rendered with `||` fallbacks).
  defaultCurrency?: string
  region?: string
  defaultRegion?: string
}

export async function fetchCurrentStore(org?: string | null): Promise<CurrentStore | null> {
  try {
    const res = await fetch(`${API_BASE}/v1/store/current`, { headers: headers(org) })
    if (!res.ok) return null
    const body = await res.json()
    const store = (body?.store ?? body) as CurrentStore
    _storeId = store?.id || null
    return store
  } catch {
    return null
  }
}

// ── Models (the Hanzo catalog: our "products" are the models) ─────────────────
// GET /v1/models (OpenAI-compatible) — org-scoped by the bearer/X-Org-Id — returns
// every model with INLINE per-model pricing { input, output } in $ / 1M tokens.
export interface ModelPricing {
  input?: number
  output?: number
}
export interface HanzoModel {
  id: string
  object?: string
  created?: number
  owned_by?: string
  provider?: string
  premium?: boolean
  pricing?: ModelPricing
}

export async function fetchModels(org?: string | null): Promise<HanzoModel[]> {
  try {
    const res = await fetch(`${API_BASE}/v1/models`, { headers: headers(org) })
    if (!res.ok) return []
    const body = await res.json()
    // OpenAI-style: `data` is the array; `models` may be a non-array alias — pick the array.
    const arr = Array.isArray(body?.data)
      ? body.data
      : Array.isArray(body?.models)
        ? body.models
        : Array.isArray(body)
          ? body
          : []
    return arr as HanzoModel[]
  } catch {
    return []
  }
}

// ── Integrations ─────────────────────────────────────────────────────────────
export interface CommerceIntegration {
  id: string
  type: string
  enabled: boolean
  show?: boolean
  data?: unknown
  createdAt?: string
  updatedAt?: string
}

export async function fetchIntegrations(org?: string | null): Promise<CommerceIntegration[]> {
  if (!org) return []
  try {
    const res = await fetch(`${API_BASE}/v1/c/organization/${encodeURIComponent(org)}/integrations`, {
      headers: headers(org),
    })
    if (!res.ok) return []
    const body = await res.json()
    return Array.isArray(body) ? body : []
  } catch {
    return []
  }
}

export async function saveIntegration(
  integration: Pick<CommerceIntegration, 'id' | 'type' | 'enabled'> & Partial<CommerceIntegration>,
  org?: string | null,
): Promise<CommerceIntegration[]> {
  if (!org) throw new Error('Select an organization first')
  const res = await fetch(`${API_BASE}/v1/c/organization/${encodeURIComponent(org)}/integrations`, {
    method: 'PATCH',
    headers: headers(org),
    body: JSON.stringify([{ ...integration, show: integration.show ?? true }]),
  })
  if (!res.ok) throw new Error(`Failed to update integration: ${res.status}`)
  const body = await res.json()
  return Array.isArray(body) ? body : []
}
