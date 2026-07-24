import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the sales-channel resource (/v1/saleschannel). Mirrors the Go
// model (camelCase: name, description, isDisabled). The form works in terms of an
// `enabled` toggle — the friendlier inverse of the stored `isDisabled` flag — and
// the mappers translate at the API boundary.

export interface SalesChannel {
  id: string
  name: string
  description?: string
  isDisabled?: boolean
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  description: z.string().trim(),
  enabled: z.boolean(),
})

export type SalesChannelValues = z.infer<typeof schema>

const fields: FieldSpec<SalesChannelValues>[] = [
  { name: 'name', label: 'Name', placeholder: 'Web store' },
  { name: 'description', label: 'Description', kind: 'textarea', optional: true, placeholder: 'Where these products are sold' },
  { name: 'enabled', label: 'Enabled', kind: 'switch' },
]

export const salesChannelDescriptor: ResourceDescriptor<SalesChannel, SalesChannelValues> = {
  kind: 'saleschannel',
  label: 'Sales channel',
  listPath: '/sales-channels',
  schema,
  empty: { name: '', description: '', enabled: true },
  fields,
  toValues: (r) => ({ name: r.name ?? '', description: r.description ?? '', enabled: !r.isDisabled }),
  toPayload: (v) => ({ name: v.name.trim(), description: v.description.trim(), isDisabled: !v.enabled }),
  recordTitle: (r) => r.name || 'Sales channel',
  deleteDescription:
    'This permanently removes the sales channel. Products stay in your catalog — they are just no longer sold here.',
}
