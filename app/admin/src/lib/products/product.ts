// Product domain — the single source of truth the product forms/pages share.
// Mirrors the live `/v1/product` model (models/product/product.go): a product
// carries its variants, options, price (integer cents), currency, media and
// status INLINE, so one PATCH edits the whole thing. Everything here is pure
// (no React) so it unit-tests and imports anywhere.
import { z } from 'zod'

// ── Wire types (subset of the Go model we read/write) ────────────────────────
export interface Media {
  type?: string
  url?: string
  alt?: string
}

export interface ProductOption {
  name: string
  values: string[]
}

export interface Variant {
  id?: string
  productId?: string
  sku?: string
  upc?: string
  name?: string
  price?: number // integer cents
  currency?: string
  available?: boolean
}

export interface Product {
  id: string
  slug: string
  sku?: string
  upc?: string
  name: string
  headline?: string
  excerpt?: string
  description?: string
  header?: Media
  image?: Media
  media?: Media[]
  available?: boolean
  hidden?: boolean
  preorder?: boolean
  currency?: string
  price?: number // integer cents
  msrp?: number // integer cents
  taxable?: boolean
  variants?: Variant[]
  options?: ProductOption[]
  createdAt?: string
  updatedAt?: string
}

export interface Collection {
  id: string
  slug: string
  name: string
  productIds?: string[]
}

// ── Status (derived, one place) ──────────────────────────────────────────────
export type ProductStatus = 'live' | 'draft' | 'hidden'

export function statusOf(p: Pick<Product, 'available' | 'hidden'>): ProductStatus {
  if (p.hidden) return 'hidden'
  return p.available ? 'live' : 'draft'
}

export const STATUS_COLOR: Record<ProductStatus, 'green' | 'grey' | 'orange'> = {
  live: 'green',
  draft: 'grey',
  hidden: 'orange',
}

// ── Money (integer cents ⇄ decimal string, one place) ────────────────────────
export const CURRENCIES = ['USD', 'EUR', 'GBP', 'CAD', 'AUD', 'JPY'] as const
export type Currency = (typeof CURRENCIES)[number]

const SYMBOL: Record<string, string> = {
  USD: '$', CAD: '$', AUD: '$', EUR: '€', GBP: '£', JPY: '¥',
}
export function symbolFor(currency: string): string {
  return SYMBOL[currency?.toUpperCase()] ?? '$'
}

/** "12.34" → 1234 cents. Blank/invalid → 0. */
export function toCents(value: string): number {
  const n = Number(String(value).trim())
  if (!isFinite(n) || n < 0) return 0
  return Math.round(n * 100)
}

/** 1234 cents → "12.34". Nullish → "". */
export function fromCents(cents?: number): string {
  if (cents == null) return ''
  return (cents / 100).toFixed(2)
}

export function formatMoney(cents: number | undefined, currency = 'USD'): string {
  if (cents == null) return '—'
  try {
    return new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(cents / 100)
  } catch {
    return `${symbolFor(currency)}${(cents / 100).toFixed(2)}`
  }
}

export function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

// A money field: blank OR a non-negative decimal.
const moneyString = z
  .string()
  .refine((v) => v.trim() === '' || (isFinite(Number(v)) && Number(v) >= 0), {
    message: 'Enter a valid amount',
  })

// ── Form schema (react-hook-form values are all strings/booleans/arrays) ──────
export const productSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required'),
  sku: z.string().min(1, 'SKU is required'),
  upc: z.string(),
  headline: z.string(),
  description: z.string(),
  available: z.boolean(),
  hidden: z.boolean(),
  preorder: z.boolean(),
  taxable: z.boolean(),
  currency: z.string().min(1),
  price: moneyString,
  msrp: moneyString,
  imageUrl: z.string(),
  headerUrl: z.string(),
  gallery: z.string(), // one image URL per line
  options: z.array(
    z.object({
      name: z.string().min(1, 'Option name is required'),
      values: z.string(), // comma-separated
    }),
  ),
  variants: z.array(
    z.object({
      name: z.string().min(1, 'Variant name is required'),
      sku: z.string(),
      price: moneyString,
      available: z.boolean(),
    }),
  ),
})

export type ProductFormValues = z.infer<typeof productSchema>

// ── Empty / defaults ─────────────────────────────────────────────────────────
export function emptyForm(): ProductFormValues {
  return {
    name: '', slug: '', sku: '', upc: '',
    headline: '', description: '',
    available: false, hidden: false, preorder: false, taxable: true,
    currency: 'USD', price: '', msrp: '',
    imageUrl: '', headerUrl: '', gallery: '',
    options: [],
    variants: [],
  }
}

const splitLines = (s: string): string[] =>
  s.split(/[\n,]/).map((v) => v.trim()).filter(Boolean)

// ── Product ⇄ form (the two adapters, one place) ──────────────────────────────
export function productToForm(p: Product): ProductFormValues {
  return {
    name: p.name ?? '',
    slug: p.slug ?? '',
    sku: p.sku ?? '',
    upc: p.upc ?? '',
    headline: p.headline ?? '',
    description: p.description ?? '',
    available: !!p.available,
    hidden: !!p.hidden,
    preorder: !!p.preorder,
    taxable: p.taxable ?? true,
    currency: p.currency || 'USD',
    price: fromCents(p.price),
    msrp: fromCents(p.msrp),
    imageUrl: p.image?.url ?? '',
    headerUrl: p.header?.url ?? '',
    gallery: (p.media ?? []).map((m) => m.url).filter(Boolean).join('\n'),
    options: (p.options ?? []).map((o) => ({
      name: o.name ?? '',
      values: (o.values ?? []).join(', '),
    })),
    variants: (p.variants ?? []).map((v) => ({
      name: v.name ?? '',
      sku: v.sku ?? '',
      price: fromCents(v.price),
      available: !!v.available,
    })),
  }
}

const media = (url: string): Media | undefined =>
  url.trim() ? { type: 'image', url: url.trim() } : undefined

/** Form values → the PATCH/POST body for `/v1/product`. */
export function formToPayload(v: ProductFormValues): Partial<Product> {
  const currency = v.currency || 'USD'
  return {
    name: v.name.trim(),
    slug: (v.slug.trim() || slugify(v.name)),
    sku: v.sku.trim(),
    upc: v.upc.trim(),
    headline: v.headline.trim(),
    description: v.description.trim(),
    available: v.available,
    hidden: v.hidden,
    preorder: v.preorder,
    taxable: v.taxable,
    currency,
    price: toCents(v.price),
    msrp: toCents(v.msrp),
    image: media(v.imageUrl),
    header: media(v.headerUrl),
    media: splitLines(v.gallery).map((url) => ({ type: 'image', url })),
    options: v.options.map((o) => ({ name: o.name.trim(), values: splitLines(o.values) })),
    variants: v.variants.map((vt) => ({
      name: vt.name.trim(),
      sku: vt.sku.trim(),
      price: toCents(vt.price),
      currency,
      available: vt.available,
    })),
  }
}
