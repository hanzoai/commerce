'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { DataTableShell } from '@/components/common/data-table-shell'

interface InventoryItem {
  id: string
  title: string
  sku: string
  material?: string
  requiresShipping: boolean
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
  col.accessor('material', {
    header: 'Material',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('requiresShipping', {
    header: 'Shipping',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ? 'Required' : 'Digital'}</span>,
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
      kind="inventoryitem"
      title="Inventory"
      description="Manage sellable inventory items"
      columns={columns}
    />
  )
}
