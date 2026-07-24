// Discount domain — the single source of truth the discount forms/pages share.
// A "discount" is the commerce `promotion` resource (bare CRUD at /v1/promotion,
// models/promotion/promotion.go): flat code/type/status/dates. The discount VALUE
// (percentage vs fixed amount, currency, allocation, target) has no column, so it
// rides in the promotion's `metadata` map — which round-trips through the API and
// reads back in the single promotion GET (no second fetch on the detail page).
// `applicationMethodId` links the projection row the backend Evaluate engine reads.
// Everything here is pure (no React) so it unit-tests and imports anywhere. Form
// values are all strings/booleans (coerced in the mappers), matching lib/products.
import { z } from 'zod'

// ── Option sets (one definition, consumed by the form + validation) ───────────
export const STATUSES = ['draft', 'active', 'inactive'] as const
export const PROMO_TYPES = ['standard', 'buyget'] as const
export const VALUE_TYPES = ['percentage', 'fixed'] as const
export const ALLOCATIONS = ['each', 'across', 'once'] as const
export const TARGET_TYPES = ['items', 'order', 'shipping_methods'] as const

export type DiscountStatus = (typeof STATUSES)[number]
export type PromoType = (typeof PROMO_TYPES)[number]
export type ValueType = (typeof VALUE_TYPES)[number]

export const STATUS_OPTIONS = [
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
] as const

export const TYPE_OPTIONS = [
  { value: 'standard', label: 'Standard' },
  { value: 'buyget', label: 'Buy X get Y' },
] as const

export const VALUE_TYPE_OPTIONS = [
  { value: 'percentage', label: 'Percentage' },
  { value: 'fixed', label: 'Fixed amount' },
] as const

export const ALLOCATION_OPTIONS = [
  { value: 'each', label: 'Each — every matching item' },
  { value: 'across', label: 'Across — split over matching items' },
  { value: 'once', label: 'Once — a single time' },
] as const

export const TARGET_TYPE_OPTIONS = [
  { value: 'items', label: 'Specific items' },
  { value: 'order', label: 'Entire order' },
  { value: 'shipping_methods', label: 'Shipping methods' },
] as const

export const CURRENCIES = ['USD', 'EUR', 'GBP', 'CAD', 'AUD', 'JPY'] as const
export const CURRENCY_OPTIONS = CURRENCIES.map((c) => ({ value: c, label: c }))

const SYMBOL: Record<string, string> = { USD: '$', CAD: '$', AUD: '$', EUR: '€', GBP: '£', JPY: '¥' }
export function currencySymbol(code: string): string {
  return SYMBOL[(code || '').toUpperCase()] ?? '$'
}

// ── Wire types (subset of the Go models we read/write) ────────────────────────
export interface DiscountMetadata {
  valueType?: ValueType
  value?: number
  currencyCode?: string
  allocation?: string
  targetType?: string
  maxQuantity?: number | null
  applicationMethodId?: string
  [key: string]: unknown
}

export interface Promotion {
  id: string
  code: string
  type: string
  status: string
  isAutomatic: boolean
  isTaxInclusive: boolean
  campaignId?: string
  startsAt?: string | null
  endsAt?: string | null
  metadata?: DiscountMetadata
  createdAt?: string
  updatedAt?: string
}

export interface ApplicationMethodPayload {
  promotionId: string
  type: string
  value: number
  currencyCode: string
  targetType: string
  allocation: string
  maxQuantity: number
}

export interface ApplicationMethod extends ApplicationMethodPayload {
  id: string
}

// ── Form schema (react-hook-form values are all strings/booleans) ─────────────
export const discountSchema = z
  .object({
    code: z.string().min(1, 'A code is required'),
    status: z.enum(STATUSES),
    type: z.enum(PROMO_TYPES),
    isAutomatic: z.boolean(),
    isTaxInclusive: z.boolean(),
    valueType: z.enum(VALUE_TYPES),
    value: z.string().min(1, 'A value is required'),
    currencyCode: z.string().min(1, 'Pick a currency'),
    allocation: z.enum(ALLOCATIONS),
    targetType: z.enum(TARGET_TYPES),
    maxQuantity: z.string(),
    campaignId: z.string(),
    startsAt: z.string(),
    endsAt: z.string(),
  })
  .refine((v) => Number.isFinite(Number(v.value)) && Number(v.value) > 0, {
    path: ['value'],
    message: 'Enter an amount greater than 0',
  })
  .refine((v) => v.valueType !== 'percentage' || Number(v.value) <= 100, {
    path: ['value'],
    message: 'A percentage cannot exceed 100',
  })
  .refine(
    (v) =>
      v.maxQuantity.trim() === '' ||
      (Number.isInteger(Number(v.maxQuantity)) && Number(v.maxQuantity) > 0),
    { path: ['maxQuantity'], message: 'Enter a whole number greater than 0' },
  )
  .refine((v) => !v.startsAt || !v.endsAt || v.startsAt <= v.endsAt, {
    path: ['endsAt'],
    message: 'The end date must be on or after the start date',
  })

export type DiscountFormValues = z.infer<typeof discountSchema>

// ── Empty / defaults ──────────────────────────────────────────────────────────
export function emptyDiscount(): DiscountFormValues {
  return {
    code: '',
    status: 'draft',
    type: 'standard',
    isAutomatic: false,
    isTaxInclusive: false,
    valueType: 'percentage',
    value: '',
    currencyCode: 'USD',
    allocation: 'each',
    targetType: 'items',
    maxQuantity: '',
    campaignId: '',
    startsAt: '',
    endsAt: '',
  }
}

// ── Promotion ⇄ form (the two adapters, one place) ────────────────────────────
const oneOf = <T extends string>(options: readonly T[], value: unknown, fallback: T): T =>
  (options as readonly string[]).includes(value as string) ? (value as T) : fallback

const isoToDateInput = (iso?: string | null): string => {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString().slice(0, 10)
}

const dateInputToIso = (value: string): string | null => {
  if (!value) return null
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? null : d.toISOString()
}

export function promotionToForm(p: Promotion): DiscountFormValues {
  const m = p.metadata ?? {}
  return {
    code: p.code ?? '',
    status: oneOf(STATUSES, p.status, 'draft'),
    type: oneOf(PROMO_TYPES, p.type, 'standard'),
    isAutomatic: !!p.isAutomatic,
    isTaxInclusive: !!p.isTaxInclusive,
    valueType: oneOf(VALUE_TYPES, m.valueType, 'percentage'),
    value: typeof m.value === 'number' ? String(m.value) : '',
    currencyCode: (typeof m.currencyCode === 'string' && m.currencyCode) || 'USD',
    allocation: oneOf(ALLOCATIONS, m.allocation, 'each'),
    targetType: oneOf(TARGET_TYPES, m.targetType, 'items'),
    maxQuantity: typeof m.maxQuantity === 'number' ? String(m.maxQuantity) : '',
    campaignId: p.campaignId ?? '',
    startsAt: isoToDateInput(p.startsAt),
    endsAt: isoToDateInput(p.endsAt),
  }
}

/** Form values → promotion PATCH/POST body (value fields folded into metadata). */
export function formToPromotion(v: DiscountFormValues, existing?: Promotion): Partial<Promotion> {
  const maxQuantity = v.maxQuantity.trim() === '' ? null : Number(v.maxQuantity)
  return {
    code: v.code.trim(),
    type: v.type,
    status: v.status,
    isAutomatic: v.isAutomatic,
    isTaxInclusive: v.isTaxInclusive,
    campaignId: v.campaignId.trim(),
    startsAt: dateInputToIso(v.startsAt),
    endsAt: dateInputToIso(v.endsAt),
    metadata: {
      valueType: v.valueType,
      value: Number(v.value),
      currencyCode: v.currencyCode,
      allocation: v.allocation,
      targetType: v.targetType,
      maxQuantity,
      ...(existing?.metadata?.applicationMethodId
        ? { applicationMethodId: existing.metadata.applicationMethodId }
        : {}),
    },
  }
}

/** Form values → applicationmethod projection (value scaled to minor units / bps). */
export function formToApplicationMethod(
  v: DiscountFormValues,
  promotionId: string,
): ApplicationMethodPayload {
  return {
    promotionId,
    type: v.valueType,
    // percentage → basis points (15 → 1500); fixed → minor units (5.00 → 500).
    value: Math.round(Number(v.value) * 100),
    currencyCode: v.currencyCode,
    targetType: v.targetType,
    allocation: v.allocation,
    maxQuantity: v.maxQuantity.trim() === '' ? 0 : Number(v.maxQuantity),
  }
}

// ── Presentation helpers (one place) ──────────────────────────────────────────
export function formatDiscountValue(p: Promotion): string {
  const m = p.metadata ?? {}
  if (typeof m.value !== 'number') return '—'
  if (m.valueType === 'fixed') {
    const currency = m.currencyCode || 'USD'
    try {
      return new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(m.value)
    } catch {
      return `${currencySymbol(currency)}${m.value.toFixed(2)}`
    }
  }
  return `${m.value}%`
}

export function statusColor(status: string): 'green' | 'grey' | 'red' {
  if (status === 'active') return 'green'
  if (status === 'inactive') return 'red'
  return 'grey'
}
