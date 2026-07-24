import { z } from 'zod'

// Domain module for the Hanzo Commerce region resource (/v1/region;
// GET/POST /v1/region/:id/countries). Mirrors the Go model models/region
// (camelCase): name, currencyCode, automaticTaxes, taxInclusiveEnabled,
// countries[]. One place owns the shape, schema, and record<->form mappers.
// Pure (no React).

export interface Country {
  iso2: string
  iso3?: string
  numCode?: number
  name?: string
  displayName?: string
  regionId?: string
}

export interface Region {
  id: string
  name: string
  currencyCode: string
  automaticTaxes: boolean
  taxInclusiveEnabled: boolean
  countries?: Country[]
  createdAt?: string
  updatedAt?: string
}

export const CURRENCY_OPTIONS = [
  { value: 'usd', label: 'USD — US Dollar' },
  { value: 'eur', label: 'EUR — Euro' },
  { value: 'gbp', label: 'GBP — British Pound' },
  { value: 'cad', label: 'CAD — Canadian Dollar' },
  { value: 'aud', label: 'AUD — Australian Dollar' },
  { value: 'jpy', label: 'JPY — Japanese Yen' },
] as const

export const regionSchema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  currencyCode: z.string().min(1, 'Currency is required'),
  automaticTaxes: z.boolean(),
  taxInclusiveEnabled: z.boolean(),
})

export type RegionValues = z.infer<typeof regionSchema>

export const emptyRegion: RegionValues = {
  name: '',
  currencyCode: 'usd',
  automaticTaxes: true,
  taxInclusiveEnabled: false,
}

export function toValues(r: Region): RegionValues {
  return {
    name: r.name ?? '',
    currencyCode: r.currencyCode ?? 'usd',
    automaticTaxes: r.automaticTaxes ?? true,
    taxInclusiveEnabled: r.taxInclusiveEnabled ?? false,
  }
}

export function toPayload(values: RegionValues): Partial<Region> {
  return {
    name: values.name.trim(),
    currencyCode: values.currencyCode,
    automaticTaxes: values.automaticTaxes,
    taxInclusiveEnabled: values.taxInclusiveEnabled,
  }
}

/** ISO-2 country code -> POST body for /v1/region/:id/countries. */
export function countryPayload(iso2: string): Partial<Country> {
  return { iso2: iso2.trim().toLowerCase() }
}
