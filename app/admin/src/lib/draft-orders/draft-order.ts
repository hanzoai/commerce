// Draft-order domain — the single source of truth the draft-order pages/forms
// share. Mirrors the live `/v1/draft-order` + `/v1/draft-order-item` models
// (models/draftorder, models/draftorderitem): an admin builds an order for a
// customer as a set of line items, then converts it into a REAL order.
//
// The draft's total is a PROJECTION over its line items (Σ unitPrice × qty),
// never a mutable counter — the same values-not-places design the gift-card
// ledger uses. Everything here is pure (no React) so it imports anywhere.
import { z } from 'zod'
import { amountToCents } from '@/lib/format'

// ── Wire types (subset of the Go models we read/write) ───────────────────────
export interface DraftOrder {
  id: string
  customerId?: string
  email?: string
  currency: string
  status: string
  orderId?: string
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

export interface DraftOrderItem {
  id: string
  draftOrderId: string
  productId?: string
  productName?: string
  variantId?: string
  variantName?: string
  quantity: number
  unitPriceCents: number
  currency: string
  createdAt?: string
}

/** GET /v1/draft-order/:id/items — the lines plus the projected running total. */
export interface DraftOrderItems {
  draftOrderId: string
  currency: string
  items: DraftOrderItem[]
  totalCents: number
}

// ── Status (derived, one place) ──────────────────────────────────────────────
export type DraftOrderStatus = 'draft' | 'complete'

export function statusOf(d: Pick<DraftOrder, 'status'>): DraftOrderStatus {
  return d.status === 'complete' ? 'complete' : 'draft'
}

export const STATUS_COLOR: Record<DraftOrderStatus, 'orange' | 'green'> = {
  draft: 'orange',
  complete: 'green',
}

// ── Currency (lowercase ISO-4217 — the Go `currency.Type` values) ─────────────
export const CURRENCIES = ['usd', 'eur', 'gbp', 'cad', 'aud', 'jpy', 'hkd', 'sgd', 'nzd', 'aed'] as const

// ── Line-item display helpers ────────────────────────────────────────────────
export function itemName(i: DraftOrderItem): string {
  return i.variantName || i.productName || i.variantId || i.productId || 'Item'
}

export function itemTotalCents(i: DraftOrderItem): number {
  return i.unitPriceCents * i.quantity
}

/** Total projected from the lines (Σ unitPrice × qty). Falls back to the
 *  server-computed `totalCents` when present so the two never disagree. */
export function totalCents(items: DraftOrderItem[]): number {
  return items.reduce((sum, i) => sum + itemTotalCents(i), 0)
}

// ── Draft create form (react-hook-form values are all strings) ────────────────
export const draftOrderSchema = z.object({
  email: z
    .string()
    .trim()
    .refine((v) => v === '' || /.+@.+\..+/.test(v), { message: 'Enter a valid email' }),
  customerId: z.string(),
  currency: z.string().min(1, 'Currency is required'),
})

export type DraftOrderFormValues = z.infer<typeof draftOrderSchema>

export function emptyForm(): DraftOrderFormValues {
  return { email: '', customerId: '', currency: 'usd' }
}

/** Form values → POST body for `/v1/draft-order`. */
export function formToCreatePayload(v: DraftOrderFormValues): Partial<DraftOrder> {
  return {
    email: v.email.trim(),
    customerId: v.customerId.trim(),
    currency: v.currency || 'usd',
    status: 'draft',
  }
}

// ── Add-a-line-item row (the builder's inline draft state) ────────────────────
export interface LineItemRow {
  name: string
  variantId: string
  productId: string
  quantity: string
  unitPrice: string
}

export function emptyRow(): LineItemRow {
  return { name: '', variantId: '', productId: '', quantity: '1', unitPrice: '' }
}

/** Validate an add-line-item row, returning a human error or null when valid. */
export function validateRow(row: LineItemRow): string | null {
  if (!row.name.trim()) return 'Enter an item name'
  const qty = Number(row.quantity)
  if (!Number.isInteger(qty) || qty < 1) return 'Quantity must be a whole number ≥ 1'
  const price = Number(row.unitPrice)
  if (!Number.isFinite(price) || price <= 0) return 'Unit price must be greater than 0'
  return null
}

/** Row → POST body for `/v1/draft-order-item`. A variant reference wins over a
 *  bare product; the entered name is stored on whichever reference is used. */
export function rowToPayload(draftOrderId: string, row: LineItemRow): Partial<DraftOrderItem> {
  const variantId = row.variantId.trim()
  const productId = row.productId.trim()
  const name = row.name.trim()
  const payload: Partial<DraftOrderItem> = {
    draftOrderId,
    quantity: parseInt(row.quantity, 10) || 0,
    unitPriceCents: amountToCents(row.unitPrice),
  }
  if (variantId) {
    payload.variantId = variantId
    payload.variantName = name
  } else {
    if (productId) payload.productId = productId
    payload.productName = name
  }
  return payload
}
