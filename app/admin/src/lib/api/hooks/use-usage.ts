'use client'

import { useQuery } from '@tanstack/react-query'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Commerce } from '@/lib/commerce-client'
import type { UsageResponse, UsageRollup, Tier } from '@/lib/commerce-client'

export interface UsageAnalytics {
  usage: UsageResponse
  rollup: UsageRollup | null
  tier: Tier | null
}

/**
 * The org+subject usage/analytics bundle: api-usage ledger, the monthly
 * allowance rollup, and the current tier + balance breakdown — fetched in one
 * pass through the same-origin billing routes. Org-scoped (X-Org-Id) and keyed
 * per subject, so switching orgs re-reads cleanly. Each underlying fetch
 * degrades to a graceful empty value, so the query resolves (never rejects) and
 * the page renders an honest empty state instead of an error.
 */
export function useUsageAnalytics() {
  const { accessToken, user, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  // The billing usage/tier routes key on the IAM subject in `owner/name` form.
  const subject = user?.owner && user?.name ? `${user.owner}/${user.name}` : 'me'

  return useQuery<UsageAnalytics>({
    queryKey: [currentOrgId ?? '__no_org__', 'usage-analytics', subject],
    enabled: isAuthenticated && !!accessToken,
    queryFn: async () => {
      const client = new Commerce({ token: accessToken, org: currentOrgId ?? undefined })
      const [usage, rollup, tier] = await Promise.all([
        client.getUsage(subject),
        client.getUsageRollup(subject),
        client.getTier(subject),
      ])
      return { usage, rollup, tier }
    },
  })
}
