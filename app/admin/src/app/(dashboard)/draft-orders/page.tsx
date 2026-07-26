'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { useRouter } from 'next/navigation'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { statusOf, STATUS_COLOR, type DraftOrder } from '@/lib/draft-orders/draft-order'

const col = createColumnHelper<DraftOrder>()

const columns = [
  col.accessor('id', {
    header: 'Draft',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue()}</span>,
  }),
  col.accessor((d) => d.email || d.customerId || '—', {
    id: 'customer',
    header: 'Customer',
    cell: (info) => <span className="text-ui-fg-base">{info.getValue()}</span>,
  }),
  col.accessor('currency', {
    header: 'Currency',
    cell: (info) => <span className="uppercase text-ui-fg-muted">{info.getValue()}</span>,
  }),
  col.accessor((d) => statusOf(d), {
    id: 'status',
    header: 'Status',
    cell: (info) => {
      const status = info.getValue()
      return <Badge color={STATUS_COLOR[status]}>{status}</Badge>
    },
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '—'}</span>
    },
  }),
]

export default function DraftOrdersPage() {
  const router = useRouter()
  return (
    <DataTableShell<DraftOrder>
      kind="draft-order"
      title="Draft orders"
      description="Build an order for a customer, then convert it into a real order"
      columns={columns}
      detailPath="/draft-orders"
      actions={
        <Button size="small" onClick={() => router.push('/draft-orders/create')}>
          New draft order
        </Button>
      }
    />
  )
}
