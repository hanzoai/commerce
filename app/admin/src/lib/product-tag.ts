import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the product-tag resource (/v1/product-tag). One free-text
// `value` labels products for filtering and merchandising.

export interface ProductTag {
  id: string
  value: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  value: z.string().trim().min(1, 'Value is required'),
})

export type ProductTagValues = z.infer<typeof schema>

const fields: FieldSpec<ProductTagValues>[] = [
  { name: 'value', label: 'Value', placeholder: 'summer' },
]

export const productTagDescriptor: ResourceDescriptor<ProductTag, ProductTagValues> = {
  kind: 'product-tag',
  label: 'Tag',
  listPath: '/tags',
  schema,
  empty: { value: '' },
  fields,
  toValues: (r) => ({ value: r.value ?? '' }),
  toPayload: (v) => ({ value: v.value.trim() }),
  recordTitle: (r) => r.value || 'Tag',
  deleteDescription: 'This removes the tag from every product it labels.',
}
