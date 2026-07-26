'use client'

// The line-item builder on the draft-order detail page. It reads the draft's
// lines + projected total (GET /v1/draft-order/:id/items), adds a line
// (POST /v1/draft-order-item), removes one (DELETE /v1/draft-order-item/:id),
// and completes the draft into a real order (POST /v1/draft-order/:id/complete).
//
// Line items are their own kind, so a create/delete of one does NOT invalidate
// the draft-order `items` action query on its own — we invalidate every query
// tagged with the `draft-order` kind explicitly so the list, the items panel,
// and the running total all re-read after each edit.

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useQueryClient } from '@tanstack/react-query'
import { Button, IconButton, Input, Text, toast } from '@hanzo/commerce-ui'
import { Trash } from '@hanzo/commerce-icons'
import { Field, Fieldset } from '@/components/common/field'
import { useCreate, useDelete, useResourceAction, useResourceActionData } from '@/lib/api/hooks'
import { formatMoney } from '@/lib/format'
import {
  emptyRow,
  itemName,
  itemTotalCents,
  rowToPayload,
  totalCents,
  validateRow,
  type DraftOrderItem,
  type DraftOrderItems,
  type LineItemRow,
} from '@/lib/draft-orders/draft-order'

interface CompletedOrder {
  id: string
  total?: number
}

export function LineItemBuilder({ id, currency, readOnly }: { id: string; currency: string; readOnly?: boolean }) {
  const router = useRouter()
  const qc = useQueryClient()
  const { data, isLoading } = useResourceActionData<DraftOrderItems>('draft-order', id, 'items')

  const create = useCreate<DraftOrderItem>('draft-order-item')
  const del = useDelete('draft-order-item')
  const complete = useResourceAction<CompletedOrder, Record<string, never>>('draft-order', id, 'complete')

  const [row, setRow] = useState<LineItemRow>(emptyRow())
  const [error, setError] = useState('')

  // Refresh every draft-order-tagged query (list, detail, items action) so the
  // running total re-reads after a cross-kind line-item mutation.
  const refresh = () =>
    qc.invalidateQueries({
      predicate: (q) => Array.isArray(q.queryKey) && q.queryKey.includes('draft-order'),
    })

  const items = data?.items ?? []
  const total = data?.totalCents ?? totalCents(items)
  const cur = data?.currency || currency

  const set = (patch: Partial<LineItemRow>) => setRow((r) => ({ ...r, ...patch }))

  const add = async () => {
    const problem = validateRow(row)
    if (problem) {
      setError(problem)
      return
    }
    setError('')
    try {
      await create.mutateAsync(rowToPayload(id, row))
      await refresh()
      setRow(emptyRow())
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Could not add the line item'
      setError(message)
      toast.error(message)
    }
  }

  const remove = async (itemId: string) => {
    try {
      await del.mutateAsync(itemId)
      await refresh()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not remove the line item')
    }
  }

  const onComplete = async () => {
    try {
      const order = await complete.mutateAsync({})
      await refresh()
      toast.success('Draft completed into an order')
      router.push(`/orders/${order.id}`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not complete the draft order')
    }
  }

  return (
    <Fieldset
      title="Line items"
      description="Add products or variants with a quantity and unit price. The total is the sum of the lines."
    >
      {/* Existing lines */}
      {isLoading ? (
        <Text size="small" className="text-ui-fg-subtle">
          Loading line items…
        </Text>
      ) : items.length === 0 ? (
        <Text size="small" className="text-ui-fg-subtle">
          No line items yet.
        </Text>
      ) : (
        <div className="flex flex-col divide-y divide-ui-border-base">
          {items.map((it) => (
            <div key={it.id} className="flex items-center justify-between gap-x-4 py-3">
              <div className="min-w-0">
                <Text size="small" weight="plus" className="truncate text-ui-fg-base">
                  {itemName(it)}
                </Text>
                <Text size="xsmall" className="text-ui-fg-muted">
                  {it.quantity} × {formatMoney(it.unitPriceCents, cur)}
                </Text>
              </div>
              <div className="flex items-center gap-x-3">
                <Text size="small" className="text-ui-fg-base">
                  {formatMoney(itemTotalCents(it), cur)}
                </Text>
                {!readOnly && (
                  <IconButton
                    size="small"
                    variant="transparent"
                    aria-label={`Remove ${itemName(it)}`}
                    onClick={() => remove(it.id)}
                    disabled={del.isPending}
                  >
                    <Trash />
                  </IconButton>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Running total */}
      <div className="flex items-center justify-between border-t border-ui-border-base pt-3">
        <Text size="small" weight="plus" className="text-ui-fg-base">
          Total
        </Text>
        <Text size="base" weight="plus" className="text-ui-fg-base">
          {formatMoney(total, cur)}
        </Text>
      </div>

      {/* Add-a-line row */}
      {!readOnly && (
        <div className="rounded-lg border border-ui-border-base bg-ui-bg-base p-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Name" error={error}>
              <Input
                placeholder="T-shirt (Large)"
                value={row.name}
                onChange={(e) => set({ name: e.target.value })}
              />
            </Field>
            <Field label="Variant ID" optional hint="Reference a variant; blank = a bare product line.">
              <Input placeholder="var_…" value={row.variantId} onChange={(e) => set({ variantId: e.target.value })} />
            </Field>
            <Field label="Product ID" optional hint="Used when no variant is given.">
              <Input placeholder="prod_…" value={row.productId} onChange={(e) => set({ productId: e.target.value })} />
            </Field>
            <div className="grid grid-cols-2 gap-4">
              <Field label="Quantity">
                <Input
                  inputMode="numeric"
                  placeholder="1"
                  value={row.quantity}
                  onChange={(e) => set({ quantity: e.target.value })}
                />
              </Field>
              <Field label="Unit price">
                <Input
                  inputMode="decimal"
                  placeholder="0.00"
                  value={row.unitPrice}
                  onChange={(e) => set({ unitPrice: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      void add()
                    }
                  }}
                />
              </Field>
            </div>
          </div>
          <div className="mt-4 flex justify-end">
            <Button type="button" size="small" variant="secondary" onClick={add} isLoading={create.isPending}>
              Add line item
            </Button>
          </div>
        </div>
      )}

      {/* Complete */}
      {!readOnly && (
        <div className="flex items-center justify-between border-t border-ui-border-base pt-4">
          <Text size="small" className="text-ui-fg-subtle">
            Convert this draft into a real order.
          </Text>
          <Button
            type="button"
            size="small"
            onClick={onComplete}
            isLoading={complete.isPending}
            disabled={items.length === 0}
          >
            Complete order
          </Button>
        </div>
      )}
    </Fieldset>
  )
}
