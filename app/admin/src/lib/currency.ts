import { z } from 'zod'
import type { FieldValues } from 'react-hook-form'
import type { FieldSpec, SelectOption } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'
import { useList } from '@/lib/api/hooks'

// Domain module for the currency resource (/v1/currency) — the reference table
// of currencies the store accepts. It is the single source the store/settings
// and product/price currency pickers read, replacing hardcoded arrays. Enabling
// a currency = creating its row; disabling = deleting it.

export interface Currency {
  id: string
  code: string
  symbol: string
  name: string
  decimalDigits: number
  includesTax: boolean
  createdAt?: string
  updatedAt?: string
}

// Offline fallback so a picker is never empty before the list loads (or if the
// seed has not run). Mirrors the server seed's most common codes.
const FALLBACK: SelectOption[] = [
  { value: 'usd', label: 'USD — US Dollar' },
  { value: 'eur', label: 'EUR — Euro' },
  { value: 'gbp', label: 'GBP — British Pound' },
  { value: 'cad', label: 'CAD — Canadian Dollar' },
  { value: 'aud', label: 'AUD — Australian Dollar' },
  { value: 'jpy', label: 'JPY — Japanese Yen' },
]

/** One currency as a labeled select option ("USD — US Dollar"). */
function toOption(c: Currency): SelectOption {
  const code = (c.code ?? '').toUpperCase()
  return { value: c.code, label: c.name ? `${code} — ${c.name}` : code }
}

/**
 * useCurrencyOptions is the ONE source of currency select options across the
 * admin. It reads the accepted-currency reference list (/v1/currency) and falls
 * back to a small built-in set while loading or when nothing is seeded.
 */
export function useCurrencyOptions(): SelectOption[] {
  const { data } = useList<Currency>('currency', { display: 200 })
  const rows = data?.models ?? []
  if (rows.length === 0) return FALLBACK
  return rows.map(toOption)
}

/**
 * withCurrencyOptions returns a copy of `fields` where the named field (default
 * `currency`) is turned into a select driven by the accepted-currency list — so
 * a static field list stays DRY while the options stay live.
 */
export function withCurrencyOptions<V extends FieldValues>(
  fields: FieldSpec<V>[],
  options: SelectOption[],
  name = 'currency',
): FieldSpec<V>[] {
  return fields.map((f) =>
    f.name === name
      ? { ...f, kind: 'select' as const, options, placeholder: f.placeholder ?? 'Select currency' }
      : f,
  )
}

const schema = z.object({
  code: z
    .string()
    .trim()
    .min(2, 'ISO code is required')
    .max(5)
    .regex(/^[A-Za-z]+$/, 'Letters only')
    .transform((s) => s.toLowerCase()),
  name: z.string().trim().min(1, 'Name is required'),
  symbol: z.string().trim().min(1, 'Symbol is required'),
  decimalDigits: z.coerce.number().int().min(0).max(4),
  includesTax: z.boolean(),
})

export type CurrencyValues = z.infer<typeof schema>

const fields: FieldSpec<CurrencyValues>[] = [
  { name: 'code', label: 'ISO code', placeholder: 'usd' },
  { name: 'name', label: 'Name', placeholder: 'US Dollar' },
  { name: 'symbol', label: 'Symbol', placeholder: '$' },
  { name: 'decimalDigits', label: 'Decimal digits', kind: 'number', placeholder: '2' },
  { name: 'includesTax', label: 'Prices include tax', kind: 'switch' },
]

export const currencyDescriptor: ResourceDescriptor<Currency, CurrencyValues> = {
  kind: 'currency',
  label: 'Currency',
  listPath: '/currencies',
  schema,
  empty: { code: '', name: '', symbol: '', decimalDigits: 2, includesTax: false },
  fields,
  toValues: (r) => ({
    code: r.code ?? '',
    name: r.name ?? '',
    symbol: r.symbol ?? '',
    decimalDigits: r.decimalDigits ?? 2,
    includesTax: !!r.includesTax,
  }),
  toPayload: (v) => ({
    code: v.code.trim().toLowerCase(),
    name: v.name.trim(),
    symbol: v.symbol.trim(),
    decimalDigits: v.decimalDigits,
    includesTax: v.includesTax,
  }),
  recordTitle: (r) => (r.code ? r.code.toUpperCase() : 'Currency'),
  deleteDescription: 'This stops the store accepting this currency. Existing prices keep their stored code.',
}
