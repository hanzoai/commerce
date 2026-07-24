import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the role resource (/v1/role) — the team's permission grouping.
// `name` is the machine handle; `description` explains what the role grants.

export interface Role {
  id: string
  name: string
  description?: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  name: z.string().trim().min(1, 'Name is required'),
  description: z.string().trim(),
})

export type RoleValues = z.infer<typeof schema>

const fields: FieldSpec<RoleValues>[] = [
  { name: 'name', label: 'Name', placeholder: 'fulfilment' },
  { name: 'description', label: 'Description', kind: 'textarea', optional: true, placeholder: 'What this role can do' },
]

export const roleDescriptor: ResourceDescriptor<Role, RoleValues> = {
  kind: 'role',
  label: 'Role',
  listPath: '/roles',
  schema,
  empty: { name: '', description: '' },
  fields,
  toValues: (r) => ({ name: r.name ?? '', description: r.description ?? '' }),
  toPayload: (v) => ({ name: v.name.trim(), description: v.description.trim() }),
  recordTitle: (r) => r.name || 'Role',
  deleteDescription: 'This permanently removes the role. Members assigned to it lose its permissions.',
}
