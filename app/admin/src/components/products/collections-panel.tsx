'use client'

import { useMemo } from 'react'
import { Switch, Text, Badge, toast } from '@hanzo/commerce-ui'
import { useList, useUpdate } from '@/lib/api/hooks'
import { Fieldset } from '@/components/common/field'
import type { Collection } from '@/lib/products/product'

interface CollectionsPanelProps {
  productId: string
  disabled?: boolean
}

/**
 * Assign this product to collections. Collections own the membership
 * (`productIds`), so a toggle PATCHes the collection — one write, no product
 * mutation. Lazy-loaded: only mounted on the detail page.
 */
export function CollectionsPanel({ productId, disabled }: CollectionsPanelProps) {
  const { data, isLoading } = useList<Collection>('collection', { display: 100 })
  const update = useUpdate<Collection>('collection')

  const collections = useMemo(() => data?.models ?? [], [data])

  const toggle = async (collection: Collection, next: boolean) => {
    const current = collection.productIds ?? []
    const productIds = next
      ? Array.from(new Set([...current, productId]))
      : current.filter((id) => id !== productId)
    try {
      await update.mutateAsync({ id: collection.id, data: { productIds } })
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not update collection')
    }
  }

  return (
    <Fieldset title="Collections" description="Group this product on your storefront.">
      {isLoading && <Text size="small" className="text-ui-fg-muted">Loading collections…</Text>}
      {!isLoading && collections.length === 0 && (
        <Text size="small" className="text-ui-fg-muted">No collections yet.</Text>
      )}
      <div className="flex flex-col divide-y divide-ui-border-base">
        {collections.map((collection) => {
          const member = (collection.productIds ?? []).includes(productId)
          return (
            <div key={collection.id} className="flex items-center justify-between gap-x-3 py-2.5 first:pt-0 last:pb-0">
              <div className="min-w-0">
                <Text size="small" weight="plus" className="truncate text-ui-fg-base">
                  {collection.name || collection.slug}
                </Text>
                <Text size="xsmall" className="truncate text-ui-fg-muted">/{collection.slug}</Text>
              </div>
              <div className="flex items-center gap-x-2">
                {member && <Badge size="2xsmall" color="green">In</Badge>}
                <Switch
                  checked={member}
                  disabled={disabled || update.isPending}
                  onCheckedChange={(next) => toggle(collection, next)}
                />
              </div>
            </div>
          )
        })}
      </div>
    </Fieldset>
  )
}
