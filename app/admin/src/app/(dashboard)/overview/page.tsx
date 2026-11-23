'use client'

/**
 * Overview — real counts and wallet balance for the active organization.
 *
 * Reads live Hanzo Commerce & Billing endpoints scoped by `X-Org-Id`. Each tile
 * settles independently so one empty or unavailable resource never hides the
 * rest of the organization's numbers, and clicking any tile opens that surface.
 */
import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button, XStack, YStack } from '@hanzo/gui'
import {
  CreditCard,
  FileText,
  Package,
  RefreshCw,
  Repeat,
  ShoppingBag,
  ShoppingCart,
  Store as StoreIcon,
  Wallet,
} from '@hanzogui/lucide-icons-2'
import { BackendStateCard, MetricCard, PageHeader, classifyBackend, type BackendState } from '@hanzo/ui/product'

import { currentStore, list, orgBalance, type OrgBalance, type Store } from '@/lib/commerce'
import { useActiveOrg } from '@/lib/org'

const TILES = [
  { slug: 'payments', kind: '/v1/billing/transactions', label: 'Payments', icon: CreditCard },
  { slug: 'ledger', kind: '/v1/billing/ledger', label: 'Ledger (90d)', icon: Wallet },
  { slug: 'subscriptions', kind: '/v1/billing/subscriptions', label: 'Subscriptions', icon: Repeat },
  { slug: 'orders', kind: 'order', label: 'Orders', icon: ShoppingBag },
  { slug: 'invoices', kind: '/v1/billing/invoices', label: 'Invoices', icon: FileText },
  { slug: 'carts', kind: 'cart', label: 'Carts', icon: ShoppingCart },
  { slug: 'products', kind: 'product', label: 'Products', icon: Package },
  { slug: 'stores', kind: 'store', label: 'Stores', icon: StoreIcon },
] as const

type State =
  | { phase: 'loading' }
  | { phase: 'error'; error: BackendState }
  | { phase: 'ready'; counts: (number | null)[]; store: Store | null; balance: OrgBalance | null }

function formatMoney(cents: number, currency = 'usd'): string {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency.toUpperCase() }).format(cents / 100)
  } catch {
    return `$${(cents / 100).toFixed(2)}`
  }
}

export default function OverviewPage() {
  const { currentOrgId, currentOrg } = useActiveOrg()
  const router = useRouter()
  const [state, setState] = useState<State>({ phase: 'loading' })

  const reload = useCallback(() => {
    setState({ phase: 'loading' })
    Promise.all([
      Promise.allSettled(TILES.map((t) => list(t.kind, currentOrgId).then((r) => r.count))),
      currentStore(currentOrgId).catch(() => null),
      orgBalance(currentOrgId).catch(() => null),
    ])
      .then(([settled, store, balance]) => {
        const allFailed = settled.every((s) => s.status === 'rejected')
        if (allFailed && settled[0]?.status === 'rejected') {
          setState({ phase: 'error', error: classifyBackend(settled[0].reason) })
          return
        }
        const counts = settled.map((s) => (s.status === 'fulfilled' ? s.value : null))
        setState({ phase: 'ready', counts, store, balance })
      })
      .catch((e) => setState({ phase: 'error', error: classifyBackend(e) }))
  }, [currentOrgId])

  useEffect(() => reload(), [reload])

  const orgLabel = currentOrg?.displayName || currentOrgId || 'Organization'
  const title = state.phase === 'ready' && state.store && state.store.name !== 'Default Store'
    ? state.store.name
    : `${orgLabel} · Commerce`

  return (
    <>
      <PageHeader
        title={title}
        subtitle={`Live Hanzo Commerce & Billing data for org "${currentOrgId ?? 'default'}".`}
        actions={
          <Button size="$2" icon={<RefreshCw size={15} />} onPress={reload}>
            Refresh
          </Button>
        }
      />
      {state.phase === 'error' ? (
        <BackendStateCard state={state.error} onRetry={reload} hint="endpoint · GET /v1/billing/transactions (Hanzo Commerce)" />
      ) : (
        <XStack gap="$3" flexWrap="wrap">
          <YStack minW={200} flex={1} cursor="pointer" onPress={() => router.push('/ledger')}>
            <MetricCard
              icon={<Wallet size={15} color="$color10" />}
              label="Available Balance"
              value={
                state.phase === 'ready' && state.balance
                  ? formatMoney(state.balance.available ?? state.balance.balance, state.balance.currency || 'usd')
                  : '—'
              }
            />
          </YStack>
          {TILES.map((t, i) => (
            <YStack key={t.slug} minW={200} flex={1} cursor="pointer" onPress={() => router.push(`/${t.slug}`)}>
              <MetricCard
                icon={<t.icon size={15} color="$color10" />}
                label={t.label}
                value={state.phase === 'ready' && state.counts[i] !== null ? String(state.counts[i]) : '—'}
              />
            </YStack>
          ))}
        </XStack>
      )}
    </>
  )
}
