// Gift-card domain — the single source of truth the gift-card forms/pages share.
// Mirrors the live `/v1/gift-card` model (models/giftcard/giftcard.go): a prepaid
// balance addressable by Code. The spendable balance is a PROJECTION
// (initial − Σ redemptions), never a mutable counter — so `initialBalanceCents`
// and `currency` are immutable after issue and are read-only in the edit form.
// Everything here is pure (no React) so it unit-tests and imports anywhere.
import { z } from 'zod'
import { amountToCents, centsToAmount } from '@/lib/format'

// ── Wire types (subset of the Go models we read/write) ───────────────────────
export interface GiftCard {
  id: string
  code: string
  initialBalanceCents: number
  currency: string
  regionId?: string
  orderId?: string
  disabled: boolean
  endsAt?: string | null
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

/** GET /v1/gift-card/:id/balance — the projected spendable balance. */
export interface GiftCardBalance {
  giftCardId: string
  code: string
  currency: string
  initialBalanceCents: number
  balanceCents: number
}

/** One append-only debit (or, for a reversal, a negative-amount credit) line. */
export interface GiftCardRedemption {
  id: string
  giftCardId: string
  amountCents: number
  currency: string
  orderId?: string
  idempotencyKey: string
  isReversal?: boolean
  reversesId?: string
  createdAt?: string
}

/** POST /v1/gift-card/:id/redeem response. */
export interface RedeemResult {
  redemption: GiftCardRedemption
  balanceCents: number
  giftCardId: string
}

/** POST /v1/gift-card/:id/void response. */
export interface VoidResult {
  balanceCents: number
  giftCardId: string
}

// ── Status (derived, one place) ──────────────────────────────────────────────
export type GiftCardStatus = 'active' | 'disabled' | 'expired'

export function statusOf(g: Pick<GiftCard, 'disabled' | 'endsAt'>): GiftCardStatus {
  if (g.disabled) return 'disabled'
  if (g.endsAt && new Date(g.endsAt).getTime() < Date.now()) return 'expired'
  return 'active'
}

export const STATUS_COLOR: Record<GiftCardStatus, 'green' | 'red' | 'orange'> = {
  active: 'green',
  disabled: 'red',
  expired: 'orange',
}

// ── Currency (lowercase ISO-4217 — the Go `currency.Type` values) ─────────────
export const CURRENCIES = ['usd', 'eur', 'gbp', 'cad', 'aud', 'jpy', 'hkd', 'sgd', 'nzd', 'aed'] as const
export type Currency = (typeof CURRENCIES)[number]

// ── Metadata (optional JSON object, one place) ────────────────────────────────
export function isJsonObject(text: string): boolean {
  try {
    const v = JSON.parse(text)
    return !!v && typeof v === 'object' && !Array.isArray(v)
  } catch {
    return false
  }
}

function parseMetadata(text: string): Record<string, unknown> {
  if (!text.trim()) return {}
  try {
    const v = JSON.parse(text)
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

// ── Date (ISO ⇄ <input type="datetime-local"> value, one place) ───────────────
function toDateTimeLocal(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// ── Form schema (react-hook-form values are all strings/booleans) ─────────────
const amountString = z
  .string()
  .refine((v) => {
    const n = Number(String(v).trim())
    return String(v).trim() !== '' && Number.isFinite(n) && n > 0
  }, { message: 'Enter an amount greater than 0' })

export const giftCardSchema = z.object({
  code: z.string().trim().min(1, 'Code is required'),
  currency: z.string().min(1, 'Currency is required'),
  initialBalance: amountString,
  regionId: z.string(),
  orderId: z.string(),
  disabled: z.boolean(),
  endsAt: z.string().refine((v) => v === '' || !Number.isNaN(Date.parse(v)), { message: 'Enter a valid date' }),
  metadata: z.string().refine((v) => v.trim() === '' || isJsonObject(v), { message: 'Enter a valid JSON object' }),
})

export type GiftCardFormValues = z.infer<typeof giftCardSchema>

// ── Empty / defaults ─────────────────────────────────────────────────────────
export function emptyForm(): GiftCardFormValues {
  return {
    code: '',
    currency: 'usd',
    initialBalance: '',
    regionId: '',
    orderId: '',
    disabled: false,
    endsAt: '',
    metadata: '',
  }
}

export function giftCardToForm(g: GiftCard): GiftCardFormValues {
  return {
    code: g.code ?? '',
    currency: g.currency || 'usd',
    initialBalance: centsToAmount(g.initialBalanceCents),
    regionId: g.regionId ?? '',
    orderId: g.orderId ?? '',
    disabled: !!g.disabled,
    endsAt: toDateTimeLocal(g.endsAt),
    metadata: g.metadata && Object.keys(g.metadata).length ? JSON.stringify(g.metadata, null, 2) : '',
  }
}

// ── Form ⇄ payload adapters (two, one place) ──────────────────────────────────
// The editable subset is shared: an edit PATCH must NOT touch the immutable
// code / currency / initial balance, so only create adds those.
function editablePayload(v: GiftCardFormValues): Partial<GiftCard> {
  return {
    regionId: v.regionId.trim(),
    orderId: v.orderId.trim(),
    disabled: v.disabled,
    endsAt: v.endsAt ? new Date(v.endsAt).toISOString() : null,
    metadata: parseMetadata(v.metadata),
  }
}

/** Form values → POST body for `/v1/gift-card`. */
export function formToCreatePayload(v: GiftCardFormValues): Partial<GiftCard> {
  return {
    code: v.code.trim().toUpperCase(),
    currency: v.currency || 'usd',
    initialBalanceCents: amountToCents(v.initialBalance),
    ...editablePayload(v),
  }
}

/** Form values → PATCH body for `/v1/gift-card/:id` (editable fields only). */
export function formToEditPayload(v: GiftCardFormValues): Partial<GiftCard> {
  return editablePayload(v)
}
