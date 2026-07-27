'use client'

/**
 * Overview — the store's real counts, nothing else.
 *
 * Each tile is one `/v1/<kind>` list read reduced to its envelope `count`; a store
 * with no catalog shows zeros, never a fabricated trend. A failed read renders the
 * shared honest state instead of the grid.
 */
import { useCallback, useEffect, useState } from 'react'
import { Button, XStack, YStack } from '@hanzo/gui'
import { Boxes, Package, Percent, RefreshCw, ShoppingBag, Users, Warehouse } from '@hanzogui/lucide-icons-2'
import { useOrganizations } from '@hanzo/iam/react'
import { BackendStateCard, MetricCard, PageHeader, classifyBackend, type BackendState } from '@hanzo/ui/product'

import { list, currentStore, type Store } from '@/lib/commerce'

/** The tiles, in reading order: catalog → demand → fulfilment. */
const TILES = [
  { kind: 'product', label: 'Products', icon: Package },
  { kind: 'collection', label: 'Collections', icon: Boxes },
  { kind: 'order', label: 'Orders', icon: ShoppingBag },
  { kind: 'c/user', label: 'Customers', icon: Users },
  { kind: 'promotion', label: 'Promotions', icon: Percent },
  { kind: 'stocklocation', label: 'Stock locations', icon: Warehouse },
]

type State =
  | { phase: 'loading' }
  | { phase: 'error'; error: BackendState }
  | { phase: 'ready'; counts: number[]; store: Store | null }

export default function OverviewPage() {
  const { currentOrgId } = useOrganizations()
  const [state, setState] = useState<State>({ phase: 'loading' })

  const reload = useCallback(() => {
    setState({ phase: 'loading' })
    Promise.all([
      Promise.all(TILES.map((t) => list(t.kind, currentOrgId).then((r) => r.count))),
      currentStore(currentOrgId),
    ])
      .then(([counts, store]) => setState({ phase: 'ready', counts, store }))
      .catch((e) => setState({ phase: 'error', error: classifyBackend(e) }))
  }, [currentOrgId])

  useEffect(() => reload(), [reload])

  return (
    <>
      <PageHeader
        title={state.phase === 'ready' && state.store ? state.store.name : 'Overview'}
        subtitle="Your store at a glance — real counts from the Hanzo Commerce API."
        actions={
          <Button size="$2" icon={<RefreshCw size={15} />} onPress={reload}>
            Refresh
          </Button>
        }
      />
      {state.phase === 'error' ? (
        <BackendStateCard state={state.error} onRetry={reload} hint="endpoint · GET /v1/product (Hanzo Commerce)" />
      ) : (
        <XStack gap="$3" flexWrap="wrap">
          {TILES.map((t, i) => (
            <YStack key={t.kind} minW={200} flex={1}>
              <MetricCard
                icon={<t.icon size={15} color="$color10" />}
                label={t.label}
                value={state.phase === 'ready' ? String(state.counts[i]) : '—'}
              />
            </YStack>
          ))}
        </XStack>
      )}
    </>
  )
}
