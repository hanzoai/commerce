import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the reservation resource (/v1/reservation). A reservation
// holds `quantity` of one inventory item at one stock location. Beyond CRUD it
// exposes POST /v1/reservation/:id/adjust to change the reserved quantity by a
// signed delta, surfaced as the "Adjust" panel on the detail view.

export interface Reservation {
  id: string
  inventoryItemId: string
  locationId: string
  quantity: number
  description?: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  inventoryItemId: z.string().trim().min(1, 'Inventory item is required'),
  locationId: z.string().trim().min(1, 'Location is required'),
  quantity: z.coerce.number().int('Whole units only').min(1, 'At least one unit'),
  description: z.string().trim(),
})

export type ReservationValues = z.infer<typeof schema>

const fields: FieldSpec<ReservationValues>[] = [
  { name: 'inventoryItemId', label: 'Inventory item', placeholder: 'invitem_…' },
  { name: 'locationId', label: 'Stock location', placeholder: 'sloc_…' },
  { name: 'quantity', label: 'Quantity', kind: 'number', placeholder: '1' },
  { name: 'description', label: 'Description', kind: 'textarea', optional: true },
]

export const reservationDescriptor: ResourceDescriptor<Reservation, ReservationValues> = {
  kind: 'reservation',
  label: 'Reservation',
  listPath: '/reservations',
  schema,
  empty: { inventoryItemId: '', locationId: '', quantity: 1, description: '' },
  fields,
  toValues: (r) => ({
    inventoryItemId: r.inventoryItemId ?? '',
    locationId: r.locationId ?? '',
    quantity: r.quantity ?? 1,
    description: r.description ?? '',
  }),
  toPayload: (v) => ({
    inventoryItemId: v.inventoryItemId.trim(),
    locationId: v.locationId.trim(),
    quantity: v.quantity,
    description: v.description.trim(),
  }),
  recordTitle: (r) => `Reservation ${r.inventoryItemId || r.id}`,
  deleteDescription: 'This releases the reserved stock back to available inventory.',
}
