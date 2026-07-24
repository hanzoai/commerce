'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useIam } from '@hanzo/iam/react'
import { setAccessToken } from '@/lib/api/data-provider'
import { AiChatDock } from '@/components/ai/ai-chat-dock'

/**
 * Standalone shell for the first-run wizard — no sidebar, no store-access gate
 * (the wizard is exactly where an org with no store belongs). Authenticated
 * users only: the IAM token is synced into the data provider and unauthenticated
 * visitors bounce to login. The AI dock is mounted here so the wizard's
 * "generate my catalog" shortcut can open it.
 */
export default function OnboardingLayout({ children }: { children: React.ReactNode }) {
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
    <div className="min-h-screen bg-ui-bg-base px-4 py-10 sm:py-16">
      {children}
      <AiChatDock />
    </div>
  )
}
