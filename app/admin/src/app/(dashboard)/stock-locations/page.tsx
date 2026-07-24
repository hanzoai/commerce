'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { StockLocation } from '@/lib/inventory/stock-location'

const col = createColumnHelper<StockLocation>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('city', {
    header: 'City',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('country', {
    header: 'Country',
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

export default function StockLocationsPage() {
  return (
    <DataTableShell<StockLocation>
      kind="stocklocation"
      title="Stock locations"
      description="The warehouses and stores that hold sellable inventory"
      columns={columns}
      detailPath="/stock-locations"
      actions={
        <Link href="/stock-locations/create">
          <Button size="small" variant="primary">Add location</Button>
        </Link>
      }
    />
  )
}
