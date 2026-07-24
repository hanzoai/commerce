import { z } from 'zod'

// Domain module for the Hanzo Commerce price-list resource (/v1/pricelist +
// /v1/price + /v1/pricepreference; POST /v1/pricing/calculate). Mirrors the Go
// models (camelCase):
//   models/pricelist       → title, description, status, type, startsAt, endsAt
//   models/price           → priceListId, priceSetId, currencyCode, amount(cents), minQuantity, maxQuantity
//   models/pricepreference → attribute, value, isTaxInclusive
// One place owns the shapes, schemas, and record<->form mappers. Pure (no React).

import { toDateInput, fromDateInput } from '@/lib/promotions/promotion'

export { toDateInput, fromDateInput }

export interface PriceList {
  id: string
  title: string
  description?: string
  status: string
  type: string
  startsAt?: string | null
  endsAt?: string | null
  createdAt?: string
  updatedAt?: string
}

export interface Price {
  id: string
  priceListId: string
  priceSetId?: string
  currencyCode: string
  amount: number
  minQuantity?: number
  maxQuantity?: number
}

export interface PricePreference {
  id: string
  attribute: string
  value: string
  isTaxInclusive: boolean
}

export const STATUS_OPTIONS = [
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Active' },
] as const

export const TYPE_OPTIONS = [
  { value: 'sale', label: 'Sale' },
  { value: 'override', label: 'Override' },
] as const

export const CURRENCY_OPTIONS = [
  { value: 'usd', label: 'USD' },
  { value: 'eur', label: 'EUR' },
  { value: 'gbp', label: 'GBP' },
] as const

// ── Price list ───────────────────────────────────────────────────────────────

export const priceListSchema = z.object({
  title: z.string().trim().min(1, 'Title is required'),
  description: z.string(),
  status: z.string().min(1, 'Status is required'),
  type: z.string().min(1, 'Type is required'),
  startsAt: z.string(),
  endsAt: z.string(),
})

export type PriceListValues = z.infer<typeof priceListSchema>

export const emptyPriceList: PriceListValues = {
  title: '',
  description: '',
  status: 'draft',
  type: 'sale',
  startsAt: '',
  endsAt: '',
}

export function toValues(p: PriceList): PriceListValues {
  return {
    title: p.title ?? '',
    description: p.description ?? '',
    status: p.status ?? 'draft',
    type: p.type ?? 'sale',
    startsAt: toDateInput(p.startsAt),
    endsAt: toDateInput(p.endsAt),
  }
}

export function toPayload(values: PriceListValues): Partial<PriceList> {
  return {
    title: values.title.trim(),
    description: values.description.trim(),
    status: values.status,
    type: values.type,
    startsAt: fromDateInput(values.startsAt),
    endsAt: fromDateInput(values.endsAt),
  }
}

// ── Price (a line on a price list) ────────────────────────────────────────────

export const priceSchema = z.object({
  currencyCode: z.string().min(1, 'Currency is required'),
  amount: z.string().trim().min(1, 'Amount is required'),
  minQuantity: z.string(),
  maxQuantity: z.string(),
  priceSetId: z.string(),
})

export type PriceValues = z.infer<typeof priceSchema>

export const emptyPrice: PriceValues = {
  currencyCode: 'usd',
  amount: '',
  minQuantity: '',
  maxQuantity: '',
  priceSetId: '',
}

export function priceToValues(p: Price): PriceValues {
  return {
    currencyCode: p.currencyCode ?? 'usd',
    amount: p.amount != null ? String(p.amount) : '',
    minQuantity: p.minQuantity ? String(p.minQuantity) : '',
    maxQuantity: p.maxQuantity ? String(p.maxQuantity) : '',
    priceSetId: p.priceSetId ?? '',
  }
}

export function priceToPayload(values: PriceValues, priceListId: string): Partial<Price> {
  return {
    priceListId,
    currencyCode: values.currencyCode,
    amount: Number(values.amount) || 0,
    minQuantity: Number(values.minQuantity) || 0,
    maxQuantity: Number(values.maxQuantity) || 0,
    priceSetId: values.priceSetId.trim(),
  }
}

// ── Price preference (global tax-inclusive attribute rule) ────────────────────

export const preferenceSchema = z.object({
  attribute: z.string().trim().min(1, 'Attribute is required'),
  value: z.string().trim().min(1, 'Value is required'),
  isTaxInclusive: z.boolean(),
})

export type PreferenceValues = z.infer<typeof preferenceSchema>

export const emptyPreference: PreferenceValues = {
  attribute: 'currency_code',
  value: '',
  isTaxInclusive: false,
}

export function preferenceToPayload(values: PreferenceValues): Partial<PricePreference> {
  return {
    attribute: values.attribute.trim(),
    value: values.value.trim(),
    isTaxInclusive: values.isTaxInclusive,
  }
}

// ── Calculate preview (POST /v1/pricing/calculate) ────────────────────────────

export interface CalculateItemResult {
  priceSetId: string
  amount: number
  currencyCode: string
  originalAmount?: number
  priceListId?: string
}

export interface CalculateResponse {
  items: CalculateItemResult[]
}
