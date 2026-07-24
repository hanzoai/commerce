import type { ZodType } from 'zod'
import type { FieldValues } from 'react-hook-form'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'

// One data record describes an entire simple CRUD resource surface: which API
// `kind` it lives at, how it reads/writes, the field list that drives its form,
// and the copy used in the shared chrome. The generic <ResourceCreate> and
// <ResourceEdit> engines consume it, so a new resource is a descriptor + a column
// list — never a hand-rolled form. Reservations, api-keys and the like layer
// bespoke panels on top via the `extra` slot; everything else is fully generic.
export interface ResourceDescriptor<T, V extends FieldValues> {
  /** API kind — the `/v1/{kind}` segment (e.g. 'saleschannel', 'product-tag'). */
  kind: string
  /** Singular human label, e.g. 'Sales channel'. */
  label: string
  /** The list route this resource returns to, e.g. '/sales-channels'. */
  listPath: string
  /** Validation schema for the form values. */
  schema: ZodType<V>
  /** Blank form values for the create surface. */
  empty: V
  /** The field list that renders the form (shared FieldRow specs). */
  fields: FieldSpec<V>[]
  /** API record -> controlled form values (never undefined). */
  toValues: (record: T) => V
  /** Form values -> trimmed API payload. */
  toPayload: (values: V) => Partial<T>
  /** Human title for one record (falls back to label). */
  recordTitle?: (record: T) => string
  /** Copy for the confirm-delete on the edit surface. */
  deleteLabel?: string
  deleteTitle?: string
  deleteDescription?: string
}
