import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'
import {
  stockLocationSchema,
  stockLocationFields,
  emptyStockLocation,
  toValues,
  toPayload,
  type StockLocation,
  type StockLocationValues,
} from '@/lib/inventory/stock-location'

// The stock-location settings surface (/stock-locations) reuses the EXISTING
// stock-location domain module verbatim — one schema, one set of mappers, shared
// with the top-level /inventory list. This descriptor only adapts that module's
// field list into the generic FieldSpec shape and names the settings route, so no
// validation or mapping logic is duplicated.
const fields: FieldSpec<StockLocationValues>[] = stockLocationFields.map((f) => ({
  name: f.name,
  label: f.label,
  optional: f.optional,
  placeholder: f.placeholder,
}))

export const stockLocationDescriptor: ResourceDescriptor<StockLocation, StockLocationValues> = {
  kind: 'stocklocation',
  label: 'Stock location',
  listPath: '/stock-locations',
  schema: stockLocationSchema,
  empty: emptyStockLocation,
  fields,
  toValues,
  toPayload,
  recordTitle: (r) => r.name || 'Stock location',
  deleteLabel: 'Delete location',
  deleteTitle: 'Delete stock location?',
  deleteDescription:
    'This permanently removes the location. Inventory levels tracked against it will no longer be available.',
}
