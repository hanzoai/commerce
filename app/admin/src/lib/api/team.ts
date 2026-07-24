'use client'

// Team / staff management data layer.
//
// BACKEND REALITY (investigated 2026-07): neither @hanzo/iam nor the commerce API
// exposes an org-members roster, an invite-by-email endpoint, or role assignment
// to the browser.
//   - `@hanzo/iam/react` surfaces only useIam / useOrganizations / useIamToken /
//     OrgProjectSwitcher. `useOrganizations` DERIVES orgs from the JWT sub/owner
//     claims and returns an empty `projects` list — there is no member listing.
//   - The raw IAM browser SDK exposes only getUser() / getUserInfo() / session —
//     no org-member or invite methods.
//   - The commerce backend's ONLY invite surface is the paywall access-code flow:
//       POST /v1/commerce/invite/redeem  { code }         (org admin reachable)
//       POST /v1/commerce/invite         { code, note? }  (platform-admin only)
//     `/customergroup/:id/members` are CUSTOMERS, not staff.
//
// So the member roster is projected from the one browser-available identity — the
// signed-in user, resolved from the IAM session — and the genuinely-wired write is
// the invite-code redeem. Inviting a teammate by email + assigning roles is the
// documented gap (needs a thin commerce proxy to IAM's org-members API, or an IAM
// members endpoint reachable from the browser). See useTeamMembers().

import { useMemo } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { redeemInvite } from './data-provider'
import type { RedeemInviteResult } from './data-provider'

/** The roles a teammate can hold within an organization. */
export type TeamRole = 'owner' | 'admin' | 'member'

export const TEAM_ROLES: { label: string; value: TeamRole }[] = [
  { label: 'Owner', value: 'owner' },
  { label: 'Admin', value: 'admin' },
  { label: 'Member', value: 'member' },
]

export interface TeamMember {
  id: string
  name: string
  email: string
  role: TeamRole
  /** True for the currently signed-in user. */
  you: boolean
}

export interface TeamMembersState {
  members: TeamMember[]
  isLoading: boolean
  /**
   * True when the list is projected from the local session only (the org-members
   * API is not reachable), so the UI can surface the honest "partial roster" note.
   */
  partial: boolean
}

/**
 * The org's team roster. Today this is projected from the IAM session — the one
 * member the browser can see is the signed-in user — because no org-members API is
 * reachable. `partial` is always true until that backend gap is closed.
 */
export function useTeamMembers(): TeamMembersState {
  const { user, isLoading } = useIam()

  const members = useMemo<TeamMember[]>(() => {
    if (!user) return []
    const email = user.email ?? ''
    const name = user.displayName || email || user.name || 'You'
    // The signed-in user administers their own org context, so they are its owner.
    // (A per-member role comes from IAM once the members API is exposed.)
    return [{ id: user.id || user.name || email, name, email, role: 'owner', you: true }]
  }, [user])

  return { members, isLoading, partial: true }
}

/**
 * Redeem an invite code for the current org — the genuinely-wired team action.
 * `POST /v1/commerce/invite/redeem`. Org-scoped, first-touch, idempotent.
 */
export function useRedeemInvite() {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  return useMutation<RedeemInviteResult, Error, string>({
    mutationFn: (code: string) => redeemInvite(code, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: [currentOrgId ?? '__no_org__'] }),
  })
}
