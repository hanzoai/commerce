// The notification-preferences form, as data: ONE schema + ONE switch field list,
// consumed by the settings/notifications sub-page. Preferences persist in the
// store row's `metadata.notifications` bag via the generic PATCH /v1/store/:id.
import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { StoreRecord } from './store-form'

export const notificationsSchema = z.object({
  orderPlaced: z.boolean(),
  orderFulfilled: z.boolean(),
  customerCreated: z.boolean(),
  inventoryLow: z.boolean(),
})

export type NotificationsValues = z.infer<typeof notificationsSchema>

export const notificationsFields: FieldSpec<NotificationsValues>[] = [
  { name: 'orderPlaced', label: 'Email me when an order is placed', kind: 'switch' },
  { name: 'orderFulfilled', label: 'Email me when an order is fulfilled', kind: 'switch' },
  { name: 'customerCreated', label: 'Email me when a customer signs up', kind: 'switch' },
  { name: 'inventoryLow', label: 'Email me when inventory runs low', kind: 'switch' },
]

const prefs = (store?: StoreRecord) =>
  (store?.metadata?.notifications ?? {}) as Partial<NotificationsValues>

export function notificationsDefaults(store?: StoreRecord): NotificationsValues {
  const p = prefs(store)
  return {
    orderPlaced: p.orderPlaced ?? true,
    orderFulfilled: p.orderFulfilled ?? true,
    customerCreated: p.customerCreated ?? false,
    inventoryLow: p.inventoryLow ?? true,
  }
}

/** Fold the toggles back into the PATCH payload, preserving unrelated metadata. */
export function notificationsPayload(values: NotificationsValues, store?: StoreRecord): Partial<StoreRecord> {
  return {
    metadata: {
      ...(store?.metadata ?? {}),
      notifications: values,
    },
  }
}
