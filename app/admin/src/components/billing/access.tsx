'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { useEffect, useState } from 'react'
import { Badge, Heading, Text } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Commerce, type StoreAccess } from '@/lib/commerce-client'
import { HanzoMark } from '@/components/hanzo-mark'
import { useStore } from '@/lib/api/hooks'

export function Access({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const { accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const { data: store, isLoading: storeLoading } = useStore()
  const [access, setAccess] = useState<StoreAccess | null | undefined>(undefined)

  useEffect(() => {
    if (!accessToken || !currentOrgId || !store?.id) return
    let alive = true
    const client = new Commerce({ token: accessToken, org: currentOrgId })
    const read = async () => {
      let next = await client.getStoreAccess(store.id)
      if (!alive) return
      if (next && !next.allowed && next.status === 'payment_required') {
        await client.startStoreTrial(store.id).catch(() => null)
        if (!alive) return
        next = await client.getStoreAccess(store.id)
      }
      if (alive) setAccess(next)
    }
    read()
    return () => { alive = false }
  }, [accessToken, currentOrgId, store?.id])

  if (pathname.startsWith('/billing')) return children

  const spinner = (
    <div className="flex min-h-screen items-center justify-center bg-ui-bg-base">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-ui-fg-base border-t-transparent" />
    </div>
  )

  if (storeLoading) return spinner

  // A storeless org (brand-new / onboarding) has no access gate to satisfy —
  // fall through to children so the onboarding nudge renders instead of spinning.
  if (!store?.id) return children

  if (access === undefined) return spinner

  if (access?.allowed) return children

  return (
    <div className="flex min-h-screen items-center justify-center bg-ui-bg-base p-6">
      <div className="w-full max-w-xl rounded-2xl border border-ui-border-base bg-ui-bg-subtle p-8 text-center shadow-elevation-card-rest">
        <HanzoMark className="mx-auto h-10 w-10 text-ui-fg-base" />
        <Badge color="orange" className="mt-6">Store access</Badge>
        <Heading level="h1" className="mt-4">Keep building for $20 / month</Heading>
        <Text className="mx-auto mt-3 max-w-md text-ui-fg-muted">
          Each store includes products, orders, customers, inventory, integrations, storefront tools, and the commerce assistant.
        </Text>
        <Text size="small" className="mt-3 text-ui-fg-muted">
          Your seven-day store trial starts once. After that, add a card to keep this store active.
        </Text>
        <Link
          href="/billing"
          className="mt-7 inline-flex rounded-md bg-ui-button-inverted px-5 py-2.5 text-sm font-medium text-ui-fg-on-inverted"
        >
          Add card and continue
        </Link>
      </div>
    </div>
  )
}
