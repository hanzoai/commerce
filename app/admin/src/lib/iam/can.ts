'use client'

import { useIam } from '@hanzo/iam/react'

// Client-side write gate for product mutations (the "Admin | WriteProduct"
// surface). The authoritative gate is server-side: commerce EdgeAuth +
// TokenRequired(Admin) enforce it and the data-provider surfaces a 403 as a
// failed mutation. This just hides write affordances from a read-only member so
// the UI matches what the API will allow. Org-admins and global admins can write.
export function useCanWriteProduct(): boolean {
  const { user } = useIam()
  if (!user) return false
  return !!(user.isAdmin || user.isGlobalAdmin)
}
