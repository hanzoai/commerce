import { z } from 'zod'

// Domain module for the Hanzo Commerce v2 promotion engine (/v1/promotion +
// /v1/applicationmethod + /v1/promotionrule + POST /v1/promotion/evaluate).
// Distinct from the v1 /discounts surface. Mirrors the Go models (camelCase):
//   models/promotion         → code, type, status, isAutomatic, isTaxInclusive, campaignId, startsAt, endsAt
//   models/applicationmethod → promotionId, value, currencyCode, maxQuantity, type, targetType, allocation
//   models/promotionrule     → promotionId, attribute, operator, values[]
// One place owns the shapes, the zod schemas, and the record<->form mappers so
// the create and edit surfaces never diverge. Pure (no React).

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
  createdAt?: string
  updatedAt?: string
}

export interface ApplicationMethod {
  id: string
  promotionId: string
  value: number
  currencyCode?: string
  maxQuantity?: number
  type: string
  targetType?: string
  allocation?: string
}

export interface PromotionRule {
  id: string
  promotionId: string
  attribute: string
  operator: string
  values?: string[]
}

export const STATUS_OPTIONS = [
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Inactive' },
] as const

export const TYPE_OPTIONS = [
  { value: 'standard', label: 'Standard' },
  { value: 'buyget', label: 'Buy X get Y' },
] as const

export const METHOD_TYPE_OPTIONS = [
  { value: 'percentage', label: 'Percentage' },
  { value: 'fixed', label: 'Fixed amount' },
] as const

export const TARGET_TYPE_OPTIONS = [
  { value: 'order', label: 'Entire order' },
  { value: 'items', label: 'Specific items' },
  { value: 'shipping', label: 'Shipping' },
] as const

export const ALLOCATION_OPTIONS = [
  { value: 'each', label: 'Each item' },
  { value: 'across', label: 'Across items' },
] as const

export const CURRENCY_OPTIONS = [
  { value: 'usd', label: 'USD' },
  { value: 'eur', label: 'EUR' },
  { value: 'gbp', label: 'GBP' },
] as const

// ── Promotion ────────────────────────────────────────────────────────────────

export const promotionSchema = z.object({
  code: z.string().trim().min(1, 'Code is required'),
  type: z.string().min(1, 'Type is required'),
  status: z.string().min(1, 'Status is required'),
  isAutomatic: z.boolean(),
  isTaxInclusive: z.boolean(),
  campaignId: z.string(),
  startsAt: z.string(),
  endsAt: z.string(),
})

export type PromotionValues = z.infer<typeof promotionSchema>

export const emptyPromotion: PromotionValues = {
  code: '',
  type: 'standard',
  status: 'draft',
  isAutomatic: false,
  isTaxInclusive: false,
  campaignId: '',
  startsAt: '',
  endsAt: '',
}

/** ISO timestamp -> `YYYY-MM-DD` for a <input type="date">. */
export function toDateInput(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '' : d.toISOString().slice(0, 10)
}

/** `YYYY-MM-DD` -> ISO timestamp, or null when empty. */
export function fromDateInput(value: string): string | null {
  if (!value) return null
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? null : d.toISOString()
}

export function toValues(p: Promotion): PromotionValues {
  return {
    code: p.code ?? '',
    type: p.type ?? 'standard',
    status: p.status ?? 'draft',
    isAutomatic: p.isAutomatic ?? false,
    isTaxInclusive: p.isTaxInclusive ?? false,
    campaignId: p.campaignId ?? '',
    startsAt: toDateInput(p.startsAt),
    endsAt: toDateInput(p.endsAt),
  }
}

export function toPayload(values: PromotionValues): Partial<Promotion> {
  return {
    code: values.code.trim(),
    type: values.type,
    status: values.status,
    isAutomatic: values.isAutomatic,
    isTaxInclusive: values.isTaxInclusive,
    campaignId: values.campaignId.trim(),
    startsAt: fromDateInput(values.startsAt),
    endsAt: fromDateInput(values.endsAt),
  }
}

// ── Application method (the discount value carried by a promotion) ────────────

export const methodSchema = z.object({
  type: z.string().min(1, 'Type is required'),
  value: z.string().trim().min(1, 'Value is required'),
  currencyCode: z.string(),
  targetType: z.string(),
  allocation: z.string(),
  maxQuantity: z.string(),
})

export type MethodValues = z.infer<typeof methodSchema>

export const emptyMethod: MethodValues = {
  type: 'percentage',
  value: '',
  currencyCode: 'usd',
  targetType: 'order',
  allocation: 'each',
  maxQuantity: '',
}

export function methodToValues(m: ApplicationMethod): MethodValues {
  return {
    type: m.type ?? 'percentage',
    value: m.value != null ? String(m.value) : '',
    currencyCode: m.currencyCode ?? 'usd',
    targetType: m.targetType ?? 'order',
    allocation: m.allocation ?? 'each',
    maxQuantity: m.maxQuantity ? String(m.maxQuantity) : '',
  }
}

export function methodToPayload(values: MethodValues, promotionId: string): Partial<ApplicationMethod> {
  return {
    promotionId,
    type: values.type,
    value: Number(values.value) || 0,
    currencyCode: values.type === 'fixed' ? values.currencyCode : '',
    targetType: values.targetType,
    allocation: values.allocation,
    maxQuantity: Number(values.maxQuantity) || 0,
  }
}

// ── Evaluate preview (POST /v1/promotion/evaluate) ────────────────────────────

export interface EvaluateAdjustment {
  promotionId: string
  code: string
  amount: number
  type: string
}

export interface EvaluateResponse {
  adjustments: EvaluateAdjustment[]
  totalDiscount: number
}
