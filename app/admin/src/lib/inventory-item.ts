import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the inventory-item resource (/v1/inventoryitem). Mirrors the
// Go model (camelCase: sku, title, description, requiresShipping). Each item's
// stock is tracked per location as inventory levels (/v1/inventorylevel), which
// the detail surface reads and displays.

export interface InventoryItem {
  id: string
  sku: string
  title?: string
  description?: string
  requiresShipping?: boolean
  createdAt?: string
  updatedAt?: string
}

export interface InventoryLevel {
  id: string
  inventoryItemId: string
  locationId: string
  stockedQuantity?: number
  reservedQuantity?: number
  incomingQuantity?: number
  createdAt?: string
}

const schema = z.object({
  sku: z.string().trim().min(1, 'SKU is required'),
  title: z.string().trim(),
  description: z.string().trim(),
  requiresShipping: z.boolean(),
})

export type InventoryItemValues = z.infer<typeof schema>

const fields: FieldSpec<InventoryItemValues>[] = [
  { name: 'sku', label: 'SKU', placeholder: 'SHIRT-BLK-M' },
  { name: 'title', label: 'Title', optional: true, placeholder: 'Black shirt, medium' },
  { name: 'description', label: 'Description', kind: 'textarea', optional: true },
  { name: 'requiresShipping', label: 'Requires shipping', kind: 'switch' },
]

/** The data-provider `kind` for the levels of one item, filtered client-side by
 *  inventoryItemId (the levels collection is flat at /v1/inventorylevel). */
export const inventoryLevelKind = 'inventorylevel'

export const inventoryItemDescriptor: ResourceDescriptor<InventoryItem, InventoryItemValues> = {
  kind: 'inventoryitem',
  label: 'Inventory item',
  listPath: '/inventory-items',
  schema,
  empty: { sku: '', title: '', description: '', requiresShipping: true },
  fields,
  toValues: (r) => ({
    sku: r.sku ?? '',
    title: r.title ?? '',
    description: r.description ?? '',
    requiresShipping: r.requiresShipping ?? true,
  }),
  toPayload: (v) => ({
    sku: v.sku.trim(),
    title: v.title.trim(),
    description: v.description.trim(),
    requiresShipping: v.requiresShipping,
  }),
  recordTitle: (r) => r.sku || r.title || 'Inventory item',
  deleteDescription:
    'This permanently removes the inventory item and its stock levels across every location.',
}
