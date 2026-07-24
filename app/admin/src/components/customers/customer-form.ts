// The customer form, as data: ONE schema + ONE field list, shared by the create
// page and the inline edit form. Field names are the live c/user camelCase JSON.
import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import { emailField, optionalText } from '@/lib/forms/schema'
import type { Customer } from '@/lib/api/models'

export const customerSchema = z.object({
  email: emailField,
  firstName: optionalText,
  lastName: optionalText,
  company: optionalText,
  phone: optionalText,
})

export type CustomerFormValues = z.infer<typeof customerSchema>

export const customerFields: FieldSpec<CustomerFormValues>[] = [
  { name: 'firstName', label: 'First name', optional: true },
  { name: 'lastName', label: 'Last name', optional: true },
  { name: 'email', label: 'Email', kind: 'email' },
  { name: 'company', label: 'Company', optional: true },
  { name: 'phone', label: 'Phone', kind: 'tel', optional: true },
]

export function customerDefaults(c?: Customer): CustomerFormValues {
  return {
    email: c?.email ?? '',
    firstName: c?.firstName ?? '',
    lastName: c?.lastName ?? '',
    company: c?.company ?? '',
    phone: c?.phone ?? '',
  }
}
