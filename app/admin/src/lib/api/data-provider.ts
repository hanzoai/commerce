const API_BASE = process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://commerce-api.hanzo.ai'

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

export function setAccessToken(token: string | null) {
  _accessToken = token
}

function headers(org?: string | null): HeadersInit {
  const h: HeadersInit = { 'Content-Type': 'application/json' }
  if (_accessToken) h['Authorization'] = `Bearer ${_accessToken}`
  if (org) h['X-IAM-Org'] = org
  return h
}

export async function fetchList<T>(kind: string, params?: ListParams, org?: string | null): Promise<ListResponse<T>> {
  const url = new URL(`${API_BASE}/api/v1/${kind}`)
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined) url.searchParams.set(k, String(v))
    })
  }
  const res = await fetch(url.toString(), { headers: headers(org) })
  if (!res.ok) throw new Error(`Failed to fetch ${kind}: ${res.status}`)
  return res.json()
}

export async function fetchOne<T>(kind: string, id: string, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/api/v1/${kind}/${id}`, { headers: headers(org) })
  if (!res.ok) throw new Error(`Failed to fetch ${kind}/${id}: ${res.status}`)
  return res.json()
}

export async function createOne<T>(kind: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/api/v1/${kind}`, {
    method: 'POST',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Failed to create ${kind}: ${res.status}`)
  return res.json()
}

export async function updateOne<T>(kind: string, id: string, data: Partial<T>, org?: string | null): Promise<T> {
  const res = await fetch(`${API_BASE}/api/v1/${kind}/${id}`, {
    method: 'PATCH',
    headers: headers(org),
    body: JSON.stringify(data),
  })
  if (!res.ok) throw new Error(`Failed to update ${kind}/${id}: ${res.status}`)
  return res.json()
}

export async function deleteOne(kind: string, id: string, org?: string | null): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/${kind}/${id}`, {
    method: 'DELETE',
    headers: headers(org),
  })
  if (!res.ok) throw new Error(`Failed to delete ${kind}/${id}: ${res.status}`)
}

export async function fetchCount(kind: string, org?: string | null): Promise<number> {
  const res = await fetchList(kind, { page: 1, display: 1 }, org)
  return res.count
}
