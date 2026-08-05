/**
 * The ONE Hanzo Commerce client the admin talks to.
 *
 * `api.hanzo.ai/v1/commerce/<kind>` — the commerce endpoint. Commerce is a
 * PLUGIN of hanzoai/cloud, and api.hanzo.ai is the unified door to every Hanzo
 * service, so the merchant surface is reached the same way as everything else.
 *
 * It used to call BARE `/v1/<kind>` on commerce.hanzo.ai, and nothing answered:
 * commerce.hanzo.ai serves the admin's own static bundle, and the binary behind
 * commerce-api never carried the resource routes. Every data view 404'd while
 * sign-in, catalog and billing all worked, which is why it read as a UI fault.
 *
 * Kinds stay singular and unhyphenated (`product`, `stocklocation`), because
 * that is what the resource binder names them. List responses use
 * the envelope `{ page, display, count, models[] }`. The org is resolved from the
 * bearer; `X-Org-Id` scopes it.
 *
 * Failures are THROWN with their HTTP status so `@hanzo/ui/product`'s
 * `classifyBackend` can render the honest state (402 → add credits, 403 → not
 * enabled, 404 → not mounted here). Nothing degrades to a fabricated empty list.
 */
import type { CommerceList } from '@hanzo/ui/product'

const API_BASE = process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://api.hanzo.ai'

/** Every merchant resource hangs off the commerce plugin's one prefix. */
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

/** The bearer the dashboard layout syncs from `@hanzo/iam` — ONE token source. */
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

/** One page of a resource. Tolerates the envelope or a bare array. */
export async function list<T>(kind: string, org: string | null, display = 50): Promise<CommerceList<T>> {
  const body = await call<{ count?: number; models?: T[] } | T[]>(`${V1}/${kind}?page=1&display=${display}`, org)
  if (Array.isArray(body)) return { rows: body, count: body.length }
  return { rows: body.models ?? [], count: body.count ?? body.models?.length ?? 0 }
}

export const create = <T>(kind: string, data: Partial<T>, org: string | null): Promise<T> =>
  call<T>(`${V1}/${kind}`, org, { method: 'POST', body: JSON.stringify(data) })

export const remove = (kind: string, id: string, org: string | null): Promise<void> =>
  call<void>(`${V1}/${kind}/${id}`, org, { method: 'DELETE' })

/** The caller-org's store, or `null` when it has none yet (a 404 is not a failure). */
export async function currentStore(org: string | null): Promise<Store | null> {
  try {
    const body = await call<{ store?: Store } & Store>('/v1/store/current', org)
    const store = body?.store ?? body
    return store?.id ? store : null
  } catch (e) {
    if (e instanceof CommerceError && e.status === 404) return null
    throw e
  }
}

export type Store = {
  id: string
  name: string
  slug: string
  domain?: string
  currency?: string
  createdAt?: string
}
