// Claim domain — the single source of truth the claims pages share. Mirrors the
// live `/v1/claim` model (models/claim/claim.go) + its claim items
// (models/claimitem/claimitem.go). A claim reports a problem with delivered
// order lines (damaged / wrong / missing) and resolves to a refund or a
// replacement order. The settled amount is a server projection (claimed qty ×
// order line price) — never edited in the browser. Everything here is pure (no
// React) so it unit-tests and imports anywhere.

// ── Wire types (subset of the Go models we read/write) ───────────────────────
export interface Claim {
  id: string
  orderId: string
  resolution: Resolution
  status: ClaimStatus
  reason?: string
  currencyCode: string
  amountCents: number
  refundId?: string
  replacementOrderId?: string
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

/** One claimed order line (GET /v1/claim/:id/items). */
export interface ClaimItem {
  id: string
  claimId: string
  itemId: string
  quantity: number
  reason: ClaimReason
  createdAt?: string
}

/** POST /v1/claim/:id/accept response. */
export interface AcceptResult {
  claim: Claim
  amountCents: number
  refundId?: string
  replacementOrderId?: string
}

// ── Resolution (how an accepted claim settles) ───────────────────────────────
export type Resolution = 'refund' | 'replace'
export const RESOLUTIONS: readonly Resolution[] = ['refund', 'replace'] as const
export const RESOLUTION_LABEL: Record<Resolution, string> = {
  refund: 'Refund',
  replace: 'Replacement order',
}

// ── Reason (why a line is claimed) ───────────────────────────────────────────
export type ClaimReason = 'damaged' | 'wrong_item' | 'missing' | 'other'
export const REASONS: readonly ClaimReason[] = ['damaged', 'wrong_item', 'missing', 'other'] as const
export const REASON_LABEL: Record<ClaimReason, string> = {
  damaged: 'Damaged',
  wrong_item: 'Wrong item',
  missing: 'Missing',
  other: 'Other',
}

// ── Status (server-owned lifecycle) ──────────────────────────────────────────
export type ClaimStatus = 'pending' | 'accepted' | 'rejected'
export const STATUS_COLOR: Record<ClaimStatus, 'orange' | 'green' | 'red'> = {
  pending: 'orange',
  accepted: 'green',
  rejected: 'red',
}

export function isOpen(c: Pick<Claim, 'status'>): boolean {
  return c.status === 'pending'
}

// ── Order line shape we read to price + label claimable items ────────────────
// The order's LineItem.Id() is its variantId, else productId — the SAME id the
// claim references, so we compute it identically here.
export interface OrderLine {
  productId?: string
  variantId?: string
  productName?: string
  variantName?: string
  quantity: number
  price: number
}

export interface OrderLite {
  id: string
  number?: number
  email?: string
  currency?: string
  total?: number
  items?: OrderLine[]
}

export function lineId(line: OrderLine): string {
  return line.variantId || line.productId || ''
}

export function lineLabel(line: OrderLine): string {
  const name = line.productName || line.variantName || lineId(line) || 'Item'
  return line.variantName && line.variantName !== line.productName ? `${name} — ${line.variantName}` : name
}

// ── Create payload (one place) ───────────────────────────────────────────────
export interface ClaimItemInput {
  itemId: string
  quantity: number
  reason: ClaimReason
}

export interface CreateClaimPayload {
  orderId: string
  resolution: Resolution
  reason?: string
  currencyCode?: string
  items: ClaimItemInput[]
}
