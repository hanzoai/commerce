'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { Badge } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'

interface InventoryItem {
  id: string
  title: string
  sku: string
  quantity: number
  location: string
  createdAt: string
}

const col = createColumnHelper<InventoryItem>()

const columns = [
  col.accessor('title', {
    header: 'Item',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('sku', {
    header: 'SKU',
    cell: (info) => <span className="font-mono text-sm text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('quantity', {
    header: 'Stock',
    cell: (info) => {
      const qty = info.getValue() ?? 0
      return (
        <Badge color={qty > 10 ? 'green' : qty > 0 ? 'orange' : 'red'}>
          {qty}
        </Badge>
      )
    },
  }),
  col.accessor('location', {
    header: 'Location',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Added',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function InventoryPage() {
  return (
    <DataTableShell<InventoryItem>
      kind="stocklocation"
      title="Inventory"
      description="Track stock levels across locations"
      columns={columns}
    />
  )
}
