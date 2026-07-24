import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the product-category resource (/v1/product-category). Mirrors
// the Go model (camelCase: name, handle, description, isActive, isInternal, rank,
// parentCategoryId). Categories nest — a category's children are read from
// /v1/product-category/:id/children and shown on the detail surface.

export interface ProductCategory {
  id: string
  name: string
  handle?: string
  description?: string
  isActive?: boolean
  isInternal?: boolean
  rank?: number
  parentCategoryId?: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  handle: z.string().trim(),
  description: z.string().trim(),
  isActive: z.boolean(),
  isInternal: z.boolean(),
})

export type ProductCategoryValues = z.infer<typeof schema>

const fields: FieldSpec<ProductCategoryValues>[] = [
  { name: 'name', label: 'Name', placeholder: 'Outerwear' },
  { name: 'handle', label: 'Handle', optional: true, placeholder: 'outerwear' },
  { name: 'description', label: 'Description', kind: 'textarea', optional: true, placeholder: 'What belongs in this category' },
  { name: 'isActive', label: 'Active', kind: 'switch' },
  { name: 'isInternal', label: 'Internal', kind: 'switch' },
]

/** The composed data-provider `kind` for a category's children sub-collection:
 *  useList(childrenKind(id)) → GET /v1/product-category/:id/children. */
export const childrenKind = (categoryId: string) => `product-category/${categoryId}/children`

export const categoryDescriptor: ResourceDescriptor<ProductCategory, ProductCategoryValues> = {
  kind: 'product-category',
  label: 'Category',
  listPath: '/categories',
  schema,
  empty: { name: '', handle: '', description: '', isActive: true, isInternal: false },
  fields,
  toValues: (r) => ({
    name: r.name ?? '',
    handle: r.handle ?? '',
    description: r.description ?? '',
    isActive: r.isActive ?? true,
    isInternal: r.isInternal ?? false,
  }),
  toPayload: (v) => ({
    name: v.name.trim(),
    handle: v.handle.trim(),
    description: v.description.trim(),
    isActive: v.isActive,
    isInternal: v.isInternal,
  }),
  recordTitle: (r) => r.name || 'Category',
  deleteDescription:
    'This permanently removes the category. Its products stay in your catalog and its children are re-parented.',
}
