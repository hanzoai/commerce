// The customer-group form, as data: ONE schema + ONE field list, shared by the
// create page and the inline edit form.
import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import { requiredText } from '@/lib/forms/schema'
import type { CustomerGroup } from '@/lib/api/models'

export const groupSchema = z.object({
  name: requiredText,
})

export type GroupFormValues = z.infer<typeof groupSchema>

export const groupFields: FieldSpec<GroupFormValues>[] = [
  { name: 'name', label: 'Name', placeholder: 'VIP customers' },
]

export function groupDefaults(g?: CustomerGroup): GroupFormValues {
  return { name: g?.name ?? '' }
}
