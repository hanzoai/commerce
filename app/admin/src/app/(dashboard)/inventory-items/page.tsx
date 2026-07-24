'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { InventoryItem } from '@/lib/inventory-item'

const col = createColumnHelper<InventoryItem>()

const columns = [
  col.accessor('sku', {
    header: 'SKU',
    cell: (info) => <span className="font-mono text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('title', {
    header: 'Title',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('requiresShipping', {
    header: 'Shipping',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() === false ? 'Not required' : 'Required'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function InventoryItemsPage() {
  return (
    <DataTableShell<InventoryItem>
      kind="inventoryitem"
      title="Inventory items"
      description="The stock-keeping units tracked across your locations"
      columns={columns}
      detailPath="/inventory-items"
      actions={
        <Link href="/inventory-items/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
