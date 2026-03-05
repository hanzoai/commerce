'use client'

import { Text } from '@hanzo/commerce-ui'
import { useIam } from '@hanzo/iam/react'

export function Topbar() {
  const { user, isAuthenticated } = useIam()

  return (
    <header className="sticky top-0 z-40 flex h-14 items-center justify-between border-b border-ui-border-base bg-ui-bg-base/80 px-6 backdrop-blur">
      <div />
      <div className="flex items-center gap-4">
        {isAuthenticated && user && (
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-ui-bg-component text-sm font-medium text-ui-fg-base">
              {(user.displayName || user.email)?.[0]?.toUpperCase() || '?'}
            </div>
            <Text size="small" className="text-ui-fg-muted">{user.displayName || user.email}</Text>
          </div>
        )}
      </div>
    </header>
  )
}
