'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Shell } from '@/components/layout/shell'
import { setAccessToken, setStoreId } from '@/lib/api/data-provider'
import { Access } from '@/components/billing/access'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const router = useRouter()

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

  if (isLoading) {
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
