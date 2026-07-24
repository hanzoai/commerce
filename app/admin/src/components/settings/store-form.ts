// The store-details form, as data: ONE schema + ONE field list, consumed by the
// settings/store sub-page. Composes the shared zod primitives and the ResourceForm
// engine — no bespoke form markup. Branding (logo + accent) rides in `metadata` so
// the generic PATCH /v1/store/:id carries it without a schema change server-side.
import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import { requiredText, optionalText } from '@/lib/forms/schema'
import type { CurrentStore } from '@/lib/api/data-provider'

/** The store row as we read/write it here — CurrentStore plus an untyped metadata bag. */
export type StoreRecord = CurrentStore & { metadata?: Record<string, unknown> }

export const storeSchema = z.object({
  name: requiredText,
  currency: requiredText,
  domain: optionalText,
  logoUrl: optionalText,
  accentColor: optionalText,
})

export type StoreFormValues = z.infer<typeof storeSchema>

export const storeFields: FieldSpec<StoreFormValues>[] = [
  { name: 'name', label: 'Store name', placeholder: 'Acme Store' },
  { name: 'currency', label: 'Default currency', placeholder: 'usd' },
  { name: 'domain', label: 'Storefront domain', placeholder: 'shop.acme.com', optional: true },
  { name: 'logoUrl', label: 'Logo URL', placeholder: 'https://…/logo.svg', optional: true },
  { name: 'accentColor', label: 'Accent color', placeholder: '#6d28d9', optional: true },
]

const branding = (store?: StoreRecord) => (store?.metadata?.branding ?? {}) as Record<string, unknown>

export function storeDefaults(store?: StoreRecord): StoreFormValues {
  const b = branding(store)
  return {
    name: store?.name ?? '',
    currency: store?.currency ?? store?.defaultCurrency ?? 'usd',
    domain: store?.domain ?? '',
    logoUrl: (b.logoUrl as string) ?? '',
    accentColor: (b.accentColor as string) ?? '',
  }
}

/** Fold flat form values back into the PATCH payload, preserving unrelated metadata. */
export function storePayload(values: StoreFormValues, store?: StoreRecord): Partial<StoreRecord> {
  return {
    name: values.name.trim(),
    currency: values.currency.trim().toLowerCase(),
    domain: values.domain?.trim() || undefined,
    metadata: {
      ...(store?.metadata ?? {}),
      branding: {
        ...branding(store),
        logoUrl: values.logoUrl?.trim() || undefined,
        accentColor: values.accentColor?.trim() || undefined,
      },
    },
  }
}
