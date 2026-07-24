'use client'

// Product assignment for one collection. The collection owns membership
// (`productIds`), so each toggle PATCHes THIS collection — one write, no product
// mutation (the mirror of products/collections-panel.tsx, which assigns from the
// product side). Rendered only on the detail page, below the general fields.

import { useMemo, useState } from 'react'
import { Badge, Input, Switch, Text, toast } from '@hanzo/commerce-ui'
import { useList, useUpdate } from '@/lib/api/hooks'
import { Fieldset } from '@/components/common/field'
import type { Collection } from '@/lib/collections/collection'

interface ProductRow {
  id: string
  name: string
  slug?: string
  sku?: string
  available?: boolean
}

interface CollectionProductsProps {
  collection: Collection
}

export function CollectionProducts({ collection }: CollectionProductsProps) {
  const { data, isLoading } = useList<ProductRow>('product', { display: 200 })
  const update = useUpdate<Collection>('collection')

  const [query, setQuery] = useState('')

  const productIds = useMemo(() => collection.productIds ?? [], [collection.productIds])
  const members = useMemo(() => new Set(productIds), [productIds])

  const products = useMemo(() => {
    const all = data?.models ?? []
    const needle = query.trim().toLowerCase()
    if (!needle) return all
    return all.filter(
      (p) => p.name?.toLowerCase().includes(needle) || p.sku?.toLowerCase().includes(needle),
    )
  }, [data, query])

  const toggle = async (product: ProductRow, next: boolean) => {
    const nextIds = next
      ? Array.from(new Set([...productIds, product.id]))
      : productIds.filter((id) => id !== product.id)
    try {
      await update.mutateAsync({ id: collection.id, data: { productIds: nextIds } })
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the collection')
    }
  }

  return (
    <Fieldset
      title="Products"
      description="Choose the products featured in this collection."
      actions={
        <Badge size="2xsmall" color={members.size > 0 ? 'green' : 'grey'}>
          {members.size} selected
        </Badge>
      }
    >
      <Input
        placeholder="Search products…"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
      />

      {isLoading ? (
        <Text size="small" className="text-ui-fg-muted">
          Loading products…
        </Text>
      ) : products.length === 0 ? (
        <Text size="small" className="text-ui-fg-muted">
          {(data?.models?.length ?? 0) === 0 ? 'No products yet.' : 'No products match your search.'}
        </Text>
      ) : (
        <div className="max-h-96 overflow-y-auto">
          <div className="flex flex-col divide-y divide-ui-border-base">
            {products.map((product) => {
              const member = members.has(product.id)
              return (
                <div
                  key={product.id}
                  className="flex items-center justify-between gap-x-3 py-2.5 first:pt-0 last:pb-0"
                >
                  <div className="min-w-0">
                    <Text size="small" weight="plus" className="truncate text-ui-fg-base">
                      {product.name || product.slug || product.id}
                    </Text>
                    {product.sku && (
                      <Text size="xsmall" className="truncate text-ui-fg-muted">
                        {product.sku}
                      </Text>
                    )}
                  </div>
                  <div className="flex items-center gap-x-2">
                    {member && (
                      <Badge size="2xsmall" color="green">
                        In
                      </Badge>
                    )}
                    <Switch
                      checked={member}
                      disabled={update.isPending}
                      onCheckedChange={(next) => toggle(product, next)}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </Fieldset>
  )
}
