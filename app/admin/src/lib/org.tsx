'use client'

/**
 * Multi-org context for Hanzo Commerce Admin.
 *
 * `@hanzo/iam/react`'s `useOrganizations()` derives memberships from the JWT
 * `orgs` claim. Platform admins (`owner === 'admin'` or `isAdmin`) can inspect
 * any tenant org (`hanzo`, `hanzo-inc`, `lecce`, `lux`, `zoo`, `pars`, or any
 * org returned by `GET /v1/iam/organizations`), so this provider merges the
 * caller's JWT memberships with IAM's organization directory, exposes a
 * cross-tenant `findOrgs` search callback for `OrgProjectSwitcher` / `UserMenu`,
 * and syncs `?org=<slug>` + localStorage cleanly.
 */
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useIam, useOrganizations, type OrgState } from '@hanzo/iam/react'
import type { Organization } from '@hanzo/iam'

import { listIamOrganizations, setAccessToken } from '@/lib/commerce'

const STORAGE_KEY = 'hanzo_commerce_active_org'

/** Primary Hanzo ecosystem orgs pinned first for platform admins. */
const PRIMARY_ADMIN_ORGS: readonly { name: string; displayName: string }[] = [
  { name: 'hanzo', displayName: 'Hanzo' },
  { name: 'hanzo-inc', displayName: 'Hanzo AI Inc.' },
  { name: 'lecce', displayName: 'U.S. Lecce' },
  { name: 'lux', displayName: 'Lux Network' },
  { name: 'zoo', displayName: 'Zoo Labs' },
  { name: 'pars', displayName: 'Pars Network' },
  { name: 'admin', displayName: 'Platform Admin' },
]

const ActiveOrgContext = createContext<OrgState | null>(null)

function orgFromQuery(): string | null {
  if (typeof window === 'undefined') return null
  const q = new URLSearchParams(window.location.search).get('org')?.trim()
  return q || null
}

function readStoredOrg(): string | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage.getItem(STORAGE_KEY) || null
  } catch {
    return null
  }
}

export function ActiveOrgProvider({ children }: { children: ReactNode }) {
  const iam = useIam()
  const base = useOrganizations()
  const [directory, setDirectory] = useState<Organization[]>([])
  const [selectedOrgId, setSelectedOrgId] = useState<string | null>(() => orgFromQuery() || readStoredOrg())

  // Keep the commerce HTTP client's bearer synchronized before any org fetch runs.
  useEffect(() => {
    setAccessToken(iam.accessToken)
  }, [iam.accessToken])

  const isPlatformAdmin = useMemo(() => {
    if (iam.user?.isAdmin || iam.user?.isGlobalAdmin) return true
    if (base.organizations.some((o) => o.name === 'admin')) return true
    return base.currentOrgId === 'admin'
  }, [iam.user, base.organizations, base.currentOrgId])

  useEffect(() => {
    if (!iam.isAuthenticated || !iam.accessToken) {
      setDirectory([])
      return
    }
    setAccessToken(iam.accessToken)
    let cancelled = false
    void listIamOrganizations()
      .then((rows) => {
        if (!cancelled) setDirectory(rows)
      })
      .catch(() => {
        /* non-admins or unavailable directory gracefully fall back to JWT orgs */
      })
    return () => {
      cancelled = true
    }
  }, [iam.isAuthenticated, iam.accessToken])

  const organizations = useMemo<Organization[]>(() => {
    const byName = new Map<string, Organization>()
    const add = (org: Organization) => {
      if (!org.name) return
      const existing = byName.get(org.name)
      byName.set(org.name, existing ? { ...existing, ...org, displayName: org.displayName || existing.displayName } : org)
    }
    if (isPlatformAdmin) {
      for (const p of PRIMARY_ADMIN_ORGS) {
        add({ owner: 'admin', name: p.name, displayName: p.displayName })
      }
    }
    for (const o of base.organizations) add(o)
    for (const o of directory) add(o)
    if (selectedOrgId && !byName.has(selectedOrgId)) {
      add({ owner: 'admin', name: selectedOrgId, displayName: selectedOrgId })
    }
    return Array.from(byName.values())
  }, [isPlatformAdmin, base.organizations, directory, selectedOrgId])

  const currentOrgId = useMemo<string | null>(() => {
    if (selectedOrgId) return selectedOrgId
    if (isPlatformAdmin && (!base.currentOrgId || base.currentOrgId === 'admin')) return 'hanzo'
    return base.currentOrgId || organizations[0]?.name || null
  }, [selectedOrgId, isPlatformAdmin, base.currentOrgId, organizations])

  const currentOrg = useMemo<Organization | null>(
    () => (currentOrgId ? (organizations.find((o) => o.name === currentOrgId) ?? { owner: 'admin', name: currentOrgId, displayName: currentOrgId }) : null),
    [currentOrgId, organizations],
  )

  const switchOrg = useCallback(
    (orgId: string) => {
      const clean = orgId.trim()
      if (!clean) return
      setSelectedOrgId(clean)
      try {
        window.localStorage.setItem(STORAGE_KEY, clean)
      } catch {
        /* ignore storage restrictions */
      }
      if (base.organizations.some((o) => o.name === clean)) {
        base.switchOrg(clean)
      }
      if (typeof window !== 'undefined') {
        const url = new URL(window.location.href)
        url.searchParams.set('org', clean)
        window.history.replaceState(null, '', url.toString())
      }
    },
    [base],
  )

  const findOrgs = useCallback(
    async (query: string): Promise<Organization[]> => {
      const q = query.trim().toLowerCase()
      const matches = !q
        ? organizations
        : organizations.filter(
            (o) =>
              o.name.toLowerCase().includes(q) ||
              (o.displayName ?? '').toLowerCase().includes(q),
          )
      if (q && isPlatformAdmin && !matches.some((o) => o.name.toLowerCase() === q)) {
        return [...matches, { owner: 'admin', name: q, displayName: q }]
      }
      return matches
    },
    [organizations, isPlatformAdmin],
  )

  const value = useMemo<OrgState>(
    () => ({
      ...base,
      organizations,
      currentOrgId,
      currentOrg,
      switchOrg,
      findOrgs: isPlatformAdmin || directory.length > 0 ? findOrgs : base.findOrgs,
    }),
    [base, organizations, currentOrgId, currentOrg, switchOrg, isPlatformAdmin, directory.length, findOrgs],
  )

  return <ActiveOrgContext.Provider value={value}>{children}</ActiveOrgContext.Provider>
}

export function useActiveOrg(): OrgState {
  const ctx = useContext(ActiveOrgContext)
  const fallback = useOrganizations()
  return ctx ?? fallback
}
