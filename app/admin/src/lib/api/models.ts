// Shared TS shapes for the merchant resources these admin pages read/write.
// Field names are the live API's camelCase JSON (Go structs → camelCase), NOT the
// Medusa snake_case the ported reference forms used. Source of truth:
//   c/user      → models/user/user.go        (email, firstName, lastName, company, phone, …)
//   customergroup → models/customergroup     (name, metadata)
//   membership  → models/customergroupmembership (customerGroupId, userId)

export interface Customer {
  id: string
  email: string
  firstName?: string
  lastName?: string
  company?: string
  phone?: string
  username?: string
  enabled?: boolean
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

export interface CustomerGroup {
  id: string
  name: string
  metadata?: Record<string, unknown>
  createdAt?: string
  updatedAt?: string
}

export interface Membership {
  id: string
  customerGroupId: string
  userId: string
  createdAt?: string
}

/** The composed data-provider `kind` for a group's members sub-collection.
 *  Reuses the generic CRUD hooks verbatim:
 *    useList(memberKind(id))                → GET    /v1/customergroup/:id/members
 *    useCreate(memberKind(id)).mutate({..}) → POST   /v1/customergroup/:id/members
 *    useDelete(memberKind(id)).mutate(uid)  → DELETE /v1/customergroup/:id/members/:uid
 */
export const memberKind = (groupId: string) => `customergroup/${groupId}/members`

/** Human display name for a customer, falling back to email then id. */
export function customerName(c: Pick<Customer, 'firstName' | 'lastName' | 'email' | 'id'>): string {
  const name = [c.firstName, c.lastName].filter(Boolean).join(' ').trim()
  return name || c.email || c.id
}
