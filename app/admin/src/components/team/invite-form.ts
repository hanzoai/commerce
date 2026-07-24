// The invite-a-teammate form, as data: ONE schema + ONE field list, driven through
// the shared ResourceForm engine (email text field + role select). Kept separate
// from the page so the same spec can back a future edit/re-invite surface.
import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import { emailField } from '@/lib/forms/schema'
import { TEAM_ROLES } from '@/lib/api/team'

export const inviteSchema = z.object({
  email: emailField,
  role: z.enum(['owner', 'admin', 'member']),
})

export type InviteFormValues = z.infer<typeof inviteSchema>

export const inviteFields: FieldSpec<InviteFormValues>[] = [
  { name: 'email', label: 'Email', kind: 'email', placeholder: 'teammate@company.com', autoComplete: 'off' },
  { name: 'role', label: 'Role', kind: 'select', placeholder: 'Select a role', options: TEAM_ROLES },
]

export function inviteDefaults(): InviteFormValues {
  return { email: '', role: 'member' }
}
