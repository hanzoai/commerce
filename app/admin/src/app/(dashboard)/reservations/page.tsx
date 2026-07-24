'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { Reservation } from '@/lib/reservation'

const col = createColumnHelper<Reservation>()

const columns = [
  col.accessor('inventoryItemId', {
    header: 'Inventory item',
    cell: (info) => <span className="font-mono text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('locationId', {
    header: 'Location',
    cell: (info) => <span className="font-mono text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('quantity', {
    header: 'Quantity',
    cell: (info) => <span className="text-ui-fg-base">{info.getValue() ?? '-'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function ReservationsPage() {
  return (
    <DataTableShell<Reservation>
      kind="reservation"
      title="Reservations"
      description="Stock held against open orders, by item and location"
      columns={columns}
      detailPath="/reservations"
      actions={
        <Link href="/reservations/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
