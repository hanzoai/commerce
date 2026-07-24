'use client'

import { memo, useMemo } from 'react'
import { Badge, Table, Text } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { formatMoney } from '@/lib/format'
import { lineItemName, lineItemSku, type Order } from './types'

// Read-only order summary: the line items table + the money breakdown. Memoized —
// it re-renders only when the order object changes, not on unrelated page state
// (edit toggle, action dialogs).
export const OrderSummary = memo(function OrderSummary({ order }: { order: Order }) {
  const items = order.items ?? []

  const totals = useMemo(
    () =>
      (
        [
          { label: 'Subtotal', value: order.subtotal },
          { label: 'Discount', value: order.discount ? -order.discount : undefined },
          { label: 'Shipping', value: order.shipping },
          { label: 'Tax', value: order.tax },
          { label: 'Total', value: order.total, strong: true },
          { label: 'Paid', value: order.paid },
          { label: 'Refunded', value: order.refunded ? -order.refunded : undefined },
        ] as { label: string; value?: number; strong?: boolean }[]
      ).filter((row) => row.value != null),
    [order],
  )

  return (
    <Section title="Summary">
      <div className="px-6 py-4">
      {items.length === 0 ? (
        <Text size="small" className="text-ui-fg-subtle">
          No items on this order.
        </Text>
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Item</Table.HeaderCell>
                <Table.HeaderCell>SKU</Table.HeaderCell>
                <Table.HeaderCell className="text-right">Qty</Table.HeaderCell>
                <Table.HeaderCell className="text-right">Price</Table.HeaderCell>
                <Table.HeaderCell className="text-right">Total</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {items.map((li, index) => (
                <Table.Row key={`${li.variantId || li.productId || 'item'}-${index}`}>
                  <Table.Cell>
                    <span className="text-ui-fg-base">{lineItemName(li)}</span>
                    {li.free && (
                      <Badge color="green" className="ml-2">
                        Free
                      </Badge>
                    )}
                  </Table.Cell>
                  <Table.Cell className="text-ui-fg-subtle">{lineItemSku(li) || '—'}</Table.Cell>
                  <Table.Cell className="text-right">{li.quantity}</Table.Cell>
                  <Table.Cell className="text-right">{formatMoney(li.price, order.currency)}</Table.Cell>
                  <Table.Cell className="text-right">{formatMoney((li.price ?? 0) * li.quantity, order.currency)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </div>
      )}

      <dl className="mt-4 flex flex-col gap-2 border-t border-ui-border-base pt-4">
        {totals.map((row) => (
          <div key={row.label} className="flex items-center justify-between">
            <dt>
              <Text size="small" weight={row.strong ? 'plus' : 'regular'} className={row.strong ? 'text-ui-fg-base' : 'text-ui-fg-subtle'}>
                {row.label}
              </Text>
            </dt>
            <dd>
              <Text size="small" weight={row.strong ? 'plus' : 'regular'} className={row.strong ? 'text-ui-fg-base' : 'text-ui-fg-subtle'}>
                {formatMoney(row.value, order.currency)}
              </Text>
            </dd>
          </div>
        ))}
      </dl>
      </div>
    </Section>
  )
})
