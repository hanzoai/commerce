import { z } from 'zod'

// Domain module for the Hanzo Commerce stock-location resource (/v1/stocklocation).
// Mirrors the Go model `models/stocklocation` (flat camelCase address fields).
// One place owns the shape, the validation schema, the empty/record mappers, and
// the field list that drives the form — so the create and edit surfaces never
// diverge.

export interface StockLocation {
  id: string
  name: string
  addressLine1?: string
  addressLine2?: string
  city?: string
  province?: string
  postalCode?: string
  country?: string
  phone?: string
  createdAt?: string
  updatedAt?: string
}

export const stockLocationSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  addressLine1: z.string(),
  addressLine2: z.string(),
  city: z.string(),
  province: z.string(),
  postalCode: z.string(),
  country: z.string(),
  phone: z.string(),
})

export type StockLocationValues = z.infer<typeof stockLocationSchema>

export const emptyStockLocation: StockLocationValues = {
  name: '',
  addressLine1: '',
  addressLine2: '',
  city: '',
  province: '',
  postalCode: '',
  country: '',
  phone: '',
}

/** API record -> controlled form values (never undefined). */
export function toValues(location: StockLocation): StockLocationValues {
  return {
    name: location.name ?? '',
    addressLine1: location.addressLine1 ?? '',
    addressLine2: location.addressLine2 ?? '',
    city: location.city ?? '',
    province: location.province ?? '',
    postalCode: location.postalCode ?? '',
    country: location.country ?? '',
    phone: location.phone ?? '',
  }
}

/** Form values -> trimmed API payload. */
export function toPayload(values: StockLocationValues): Partial<StockLocation> {
  return {
    name: values.name.trim(),
    addressLine1: values.addressLine1.trim(),
    addressLine2: values.addressLine2.trim(),
    city: values.city.trim(),
    province: values.province.trim(),
    postalCode: values.postalCode.trim(),
    country: values.country.trim(),
    phone: values.phone.trim(),
  }
}

// The one field list that renders the form. `span` = full-width row.
export const stockLocationFields: {
  name: keyof StockLocationValues
  label: string
  optional?: boolean
  placeholder?: string
  span?: boolean
}[] = [
  { name: 'name', label: 'Name', placeholder: 'Main warehouse', span: true },
  { name: 'addressLine1', label: 'Address', optional: true, placeholder: '1 Market St' },
  { name: 'addressLine2', label: 'Address line 2', optional: true, placeholder: 'Suite 400' },
  { name: 'city', label: 'City', optional: true, placeholder: 'San Francisco' },
  { name: 'province', label: 'State / Province', optional: true, placeholder: 'CA' },
  { name: 'postalCode', label: 'Postal code', optional: true, placeholder: '94105' },
  { name: 'country', label: 'Country', optional: true, placeholder: 'US' },
  { name: 'phone', label: 'Phone', optional: true, placeholder: '+1 555 000 0000' },
]
