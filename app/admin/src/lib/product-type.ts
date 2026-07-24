import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the product-type resource (/v1/product-type). One free-text
// `value` classifies products (e.g. "Digital", "Apparel").

export interface ProductType {
  id: string
  value: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  value: z.string().trim().min(1, 'Value is required'),
})

export type ProductTypeValues = z.infer<typeof schema>

const fields: FieldSpec<ProductTypeValues>[] = [
  { name: 'value', label: 'Value', placeholder: 'Digital' },
]

export const productTypeDescriptor: ResourceDescriptor<ProductType, ProductTypeValues> = {
  kind: 'product-type',
  label: 'Type',
  listPath: '/types',
  schema,
  empty: { value: '' },
  fields,
  toValues: (r) => ({ value: r.value ?? '' }),
  toPayload: (v) => ({ value: v.value.trim() }),
  recordTitle: (r) => r.value || 'Type',
  deleteDescription: 'This removes the type. Products of this type keep their other details.',
}
