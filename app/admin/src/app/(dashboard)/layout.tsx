'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Shell } from '@/components/layout/shell'
import { setAccessToken, setStoreId } from '@/lib/api/data-provider'
import { Access } from '@/components/billing/access'
import { useStore } from '@/lib/api/hooks'
import { isOnboardingDismissed } from '@/lib/onboarding-state'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const router = useRouter()
  // A brand-new org has no store yet. We resolve it here so a storeless org is
  // sent straight into the guided setup wizard instead of a hollow dashboard.
  const { data: store, isLoading: storeLoading } = useStore()

  // Sync IAM token into the data provider (org is passed per-call by hooks)
  useEffect(() => {
    setAccessToken(accessToken)
  }, [accessToken])

  useEffect(() => {
    setStoreId(null)
  }, [currentOrgId])

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace('/login')
    }
  }, [isLoading, isAuthenticated, router])

  // First-run redirect: an authenticated org with NO store (and that hasn't
  // opted out of / completed onboarding) lands on the wizard automatically.
  // `store === null` means the /store/current query settled with no store —
  // `undefined` is still-loading, so we never bounce mid-fetch.
  useEffect(() => {
    if (isLoading || !isAuthenticated || storeLoading) return
    if (store === null && !isOnboardingDismissed(currentOrgId)) {
      router.replace('/onboarding')
    }
  }, [isLoading, isAuthenticated, storeLoading, store, currentOrgId, router])

  const redirecting =
    isAuthenticated && !storeLoading && store === null && !isOnboardingDismissed(currentOrgId)

  if (isLoading || (isAuthenticated && storeLoading) || redirecting) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-ui-bg-base">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-white border-t-transparent" />
      </div>
    )
  }

  if (!isAuthenticated) {
    return null
  }

  return <Access><Shell>{children}</Shell></Access>
}
