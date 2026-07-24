'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useIam } from '@hanzo/iam/react'
import { setAccessToken } from '@/lib/api/data-provider'

/**
 * Standalone shell for the paywall — no sidebar, no store-access gate (both of
 * those redirect unfunded orgs, which is exactly who lands here). Authenticated
 * users only: the IAM token is synced into the data provider and unauthenticated
 * visitors bounce to login.
 */
export default function SubscribeLayout({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, accessToken } = useIam()
  const router = useRouter()

  useEffect(() => {
    setAccessToken(accessToken)
  }, [accessToken])

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace('/login')
    }
  }, [isLoading, isAuthenticated, router])

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-ui-bg-base">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-ui-fg-base border-t-transparent" />
      </div>
    )
  }

  if (!isAuthenticated) return null

  return (
    <div className="flex min-h-screen justify-center bg-ui-bg-base px-4 py-12 sm:py-16">
      {children}
    </div>
  )
}
