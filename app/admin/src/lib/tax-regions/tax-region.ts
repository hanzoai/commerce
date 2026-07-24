import { z } from 'zod'

// Domain module for the Hanzo Commerce tax configuration (/v1/taxregion +
// /v1/taxrate + /v1/taxraterule; POST /v1/tax/calculate). Lets merchants
// CONFIGURE tax. Mirrors the Go models (camelCase):
//   models/taxregion   → countryCode, provinceCode, parentId, providerId
//   models/taxrate     → taxRegionId, rate(%), code, name, isDefault, isCombinable
//   models/taxraterule → taxRateId, reference, referenceId
// One place owns the shapes, schemas, and record<->form mappers. Pure (no React).

export interface TaxRegion {
  id: string
  countryCode: string
  provinceCode?: string
  parentId?: string
  providerId?: string
  createdAt?: string
  updatedAt?: string
}

export interface TaxRate {
  id: string
  taxRegionId: string
  rate: number
  code?: string
  name: string
  isDefault: boolean
  isCombinable: boolean
}

// ── Tax region ─────────────────────────────────────────────────────────────

export const taxRegionSchema = z.object({
  countryCode: z
    .string()
    .trim()
    .min(2, 'Use a 2-letter ISO country code')
    .max(2, 'Use a 2-letter ISO country code'),
  provinceCode: z.string(),
  providerId: z.string(),
})

export type TaxRegionValues = z.infer<typeof taxRegionSchema>

export const emptyTaxRegion: TaxRegionValues = {
  countryCode: '',
  provinceCode: '',
  providerId: '',
}

export function toValues(t: TaxRegion): TaxRegionValues {
  return {
    countryCode: t.countryCode ?? '',
    provinceCode: t.provinceCode ?? '',
    providerId: t.providerId ?? '',
  }
}

export function toPayload(values: TaxRegionValues): Partial<TaxRegion> {
  return {
    countryCode: values.countryCode.trim().toLowerCase(),
    provinceCode: values.provinceCode.trim().toLowerCase(),
    providerId: values.providerId.trim(),
  }
}

/** Human label for a tax region row. */
export function taxRegionName(t: TaxRegion): string {
  const country = (t.countryCode ?? '').toUpperCase()
  const province = (t.provinceCode ?? '').toUpperCase()
  return province ? `${country} · ${province}` : country || t.id
}

// ── Tax rate (a rate within a region) ─────────────────────────────────────────

export const taxRateSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  rate: z.string().trim().min(1, 'Rate is required'),
  code: z.string(),
  isDefault: z.boolean(),
  isCombinable: z.boolean(),
})

export type TaxRateValues = z.infer<typeof taxRateSchema>

export const emptyTaxRate: TaxRateValues = {
  name: '',
  rate: '',
  code: '',
  isDefault: false,
  isCombinable: false,
}

export function taxRateToValues(r: TaxRate): TaxRateValues {
  return {
    name: r.name ?? '',
    rate: r.rate != null ? String(r.rate) : '',
    code: r.code ?? '',
    isDefault: r.isDefault ?? false,
    isCombinable: r.isCombinable ?? false,
  }
}

export function taxRateToPayload(values: TaxRateValues, taxRegionId: string): Partial<TaxRate> {
  return {
    taxRegionId,
    name: values.name.trim(),
    rate: Number(values.rate) || 0,
    code: values.code.trim(),
    isDefault: values.isDefault,
    isCombinable: values.isCombinable,
  }
}

// ── Calculate preview (POST /v1/tax/calculate) ────────────────────────────────

export interface TaxCalcItemResult {
  amount: number
  quantity: number
  taxRate: number
  tax: number
}

export interface TaxCalcResponse {
  items: TaxCalcItemResult[]
  totalTax: number
}
