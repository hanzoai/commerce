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

// The token last set by the dashboard layout. Surfaces (e.g. the AI dock) that
// fetch outside the hooks read it here so they share ONE token source rather
// than a possibly-unhydrated `useIam().accessToken`.
export function getAccessToken(): string | null {
  return _accessToken
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

// ── Subscription paywall interceptor ─────────────────────────────────────────
// Any commerce fetch that returns 402 { code: "subscription_required" } means the
// org has no active subscription: send the browser to the paywall. Every request
// flows through `apiFetch`, so this is the ONE place the gate is enforced.
async function guardSubscription(res: Response): Promise<void> {
  if (res.status !== 402 || typeof window === 'undefined') return
  if (window.location.pathname.startsWith('/subscribe')) return
  try {
    const body = await res.clone().json()
    if (body?.code === 'subscription_required') {
      window.location.assign('/subscribe')
    }
  } catch {
    // Non-JSON 402 — leave it to the caller's own error handling.
  }
}

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await globalThis.fetch(input, init)
  await guardSubscription(res)
  return res
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
    const res = await apiFetch(url.toString(), { headers: headers(org) })
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
  const res = await apiFetch(`${API_BASE}/v1/${kind}/${id}`, { headers: headers(org) })
  if (!res.ok) throw new Error(await errorMessage(res, `Failed to fetch ${kind}/${id}: ${res.status}`))
  return res.json()
}

export async function createOne<T>(kind: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await apiFetch(`${API_BASE}/v1/${kind}`, {
    method: 'POST',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await errorMessage(res, `Failed to create ${kind}: ${res.status}`))
  return res.json()
}

// Pull the human-readable reason out of a non-2xx commerce response. The API's
// canonical envelope is NESTED — `{ "error": { "type", "message", "code" } }`
// (util/json/http Fail) — so read `error.message` first; also tolerate a flat
// `{ "message" }` or a string `{ "error" }`, then raw text, then the caller's
// default, so a real reason (e.g. "Token lacks permission to create store")
// always surfaces instead of a bare status code.
async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const text = await res.text()
    if (!text) return fallback
    try {
      const body = JSON.parse(text) as {
        message?: string
        error?: string | { message?: string }
      }
      const nested = typeof body.error === 'object' ? body.error?.message : body.error
      return body.message || nested || text
    } catch {
      return text
    }
  } catch {
    return fallback
  }
}

export async function updateOne<T>(kind: string, id: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await apiFetch(`${API_BASE}/v1/${kind}/${id}`, {
    method: 'PATCH',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(await errorMessage(res, `Failed to update ${kind}/${id}: ${res.status}`))
  return res.json()
}

export async function deleteOne(kind: string, id: string, org?: string | null): Promise<void> {
  const res = await apiFetch(`${API_BASE}/v1/${kind}/${id}`, {
    method: 'DELETE',
    headers: headers(org),
  })
  if (!res.ok) throw new Error(await errorMessage(res, `Failed to delete ${kind}/${id}: ${res.status}`))
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
  const res = await apiFetch(`${API_BASE}/v1/store/current`, { headers: headers(org) })
  // Distinguish "definitively no store" from "the fetch broke". Only a 404 (or an
  // empty 2xx body) means the org has no store → null → the layout routes to
  // onboarding. A 5xx / auth blip / network throw must NOT resolve to null: that
  // would bounce an org that HAS a store into the onboarding wizard on a transient
  // failure. Re-throw so React Query REJECTS and retries instead of caching a false
  // "storeless". (401/403 also re-throw — an unauthenticated read is not "no store".)
  if (res.status === 404) {
    _storeId = null
    return null
  }
  if (!res.ok) {
    throw new Error(`store/current ${res.status}`)
  }
  const body = await res.json()
  const store = (body?.store ?? body) as CurrentStore | null
  if (!store || !store.id) {
    _storeId = null
    return null
  }
  _storeId = store.id
  return store
}

// ── Uploads (product images → S3) ─────────────────────────────────────────────
// POST /v1/upload/images accepts multipart file(s) under the `files` field and
// returns { url, urls }. The bytes are stored under the caller-org's tenant
// prefix (tenant/<org>/uploads/) and served from the public CDN. Content-Type is
// NOT set here — the browser sets the multipart boundary itself. Admin-gated.
export interface UploadResult {
  url: string
  urls: string[]
}

export async function uploadImages(files: File[], org?: string | null): Promise<string[]> {
  if (files.length === 0) return []
  const form = new FormData()
  for (const f of files) form.append('files', f)

  // Deliberately omit Content-Type (the browser fills in the multipart
  // boundary); carry the same bearer + org scoping as every other write.
  const h: Record<string, string> = {}
  if (_accessToken) h['Authorization'] = `Bearer ${_accessToken}`
  if (org) h['X-Org-Id'] = org
  if (_storeId) h['X-Store-Id'] = _storeId

  const res = await apiFetch(`${API_BASE}/v1/upload/images`, { method: 'POST', headers: h, body: form })
  if (!res.ok) throw new Error(await errorMessage(res, `Upload failed: ${res.status}`))
  const body = (await res.json()) as UploadResult
  return body.urls ?? (body.url ? [body.url] : [])
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
    const res = await apiFetch(`${API_BASE}/v1/models`, { headers: headers(org) })
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
  data?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

// Upsert shape: `id` is absent for a first-time connect (the backend keys by
// `type`), and `data` carries the provider credentials — which the commerce API
// hydrates into Hanzo KMS at /tenants/{org}/{type}/* (never persisted in the row).
export interface IntegrationInput {
  id?: string
  type: string
  enabled: boolean
  show?: boolean
  data?: Record<string, unknown>
}

export async function fetchIntegrations(org?: string | null): Promise<CommerceIntegration[]> {
  if (!org) return []
  try {
    const res = await apiFetch(`${API_BASE}/v1/c/organization/${encodeURIComponent(org)}/integrations`, {
      headers: headers(org),
    })
    if (!res.ok) return []
    const body = await res.json()
    return Array.isArray(body) ? body : []
  } catch {
    return []
  }
}

// Upsert one integration by `type`. Enables/pauses the provider and, when `data`
// is present, syncs its credentials into KMS. Returns the org's full, refreshed
// integration list. One path for connect · configure · enable · pause.
export async function saveIntegration(
  integration: IntegrationInput,
  org?: string | null,
): Promise<CommerceIntegration[]> {
  if (!org) throw new Error('Select an organization first')
  const res = await apiFetch(`${API_BASE}/v1/c/organization/${encodeURIComponent(org)}/integrations`, {
    method: 'PATCH',
    headers: headers(org),
    body: JSON.stringify([{ ...integration, show: integration.show ?? true }]),
  })
  if (!res.ok) throw new Error(`Failed to update integration: ${res.status}`)
  const body = await res.json()
  return Array.isArray(body) ? body : []
}

// ── Team / invite codes ───────────────────────────────────────────────────────
// The ONLY team/membership surface reachable by an org admin from the browser.
// `POST /v1/commerce/invite/redeem { code }` claims a platform-minted invite code
// for the caller's org (first-touch, idempotent) — the code-based path by which a
// teammate's org unlocks SHARED store access. Org membership itself (the full
// member roster, invite-by-email, role assignment) is owned by Hanzo IAM and is
// NOT yet exposed to the browser (see team.ts for the documented gap).
export interface RedeemInviteResult {
  code?: string
  org?: string
  redeemed?: boolean
}

export async function redeemInvite(code: string, org?: string | null): Promise<RedeemInviteResult> {
  const res = await apiFetch(`${API_BASE}/v1/commerce/invite/redeem`, {
    method: 'POST',
    headers: headers(org),
    body: JSON.stringify({ code }),
  })
  if (!res.ok) throw new Error(await errorMessage(res, `Failed to redeem invite: ${res.status}`))
  return res.json()
}

// ── Sub-resource actions ──────────────────────────────────────────────────────
// Some resources expose action sub-routes beyond CRUD — e.g. a gift card's
// POST /v1/gift-card/:id/redeem|void and GET /v1/gift-card/:id/balance|redemptions.
// These two helpers are the ONE way to reach any `/v1/{kind}/{id}/{action}`
// endpoint, carrying the same bearer + org headers as the CRUD calls above.

// The API renders errors as { error: { message, code } }. Surface that message
// (e.g. "insufficient gift card balance") so callers can toast something useful,
// and carry the HTTP status for callers that branch on it (402 = out of funds).
async function actionError(res: Response, fallback: string): Promise<never> {
  let message = fallback
  try {
    const body = await res.clone().json()
    message = body?.error?.message || body?.message || fallback
  } catch {
    // Non-JSON body — keep the fallback.
  }
  const err = new Error(message) as Error & { status?: number }
  err.status = res.status
  throw err
}

export async function fetchAction<T>(kind: string, id: string, action: string, org?: string | null): Promise<T> {
  const res = await apiFetch(`${API_BASE}/v1/${kind}/${id}/${action}`, { headers: headers(org) })
  if (!res.ok) return actionError(res, `Failed to load ${kind}/${id}/${action}: ${res.status}`)
  return res.json()
}

export async function postAction<T>(kind: string, id: string, action: string, data: unknown, org?: string | null): Promise<T> {
  const res = await apiFetch(`${API_BASE}/v1/${kind}/${id}/${action}`, {
    method: 'POST',
    headers: headers(org),
    body: JSON.stringify(data ?? {}),
  })
  if (!res.ok) return actionError(res, `Failed to ${action} ${kind}/${id}: ${res.status}`)
  return res.json()
}
