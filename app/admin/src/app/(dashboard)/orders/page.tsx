'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { Badge } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'

interface Order {
  id: string
  number: string
  email: string
  total: number
  currency: string
  status: string
  fulfillmentStatus: string
  paymentStatus: string
  createdAt: string
}

const col = createColumnHelper<Order>()

const statusColor = (s: string) => {
  switch (s) {
    case 'completed': return 'green'
    case 'pending': return 'orange'
    case 'cancelled': return 'red'
    case 'refunded': return 'red'
    default: return 'grey'
  }
}

const columns = [
  col.accessor('number', {
    header: 'Order',
    cell: (info) => <span className="font-medium text-ui-fg-base">#{info.getValue() || info.row.original.id?.slice(-6)}</span>,
  }),
  col.accessor('email', {
    header: 'Customer',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('total', {
    header: 'Total',
    cell: (info) => {
      const total = info.getValue()
      const currency = info.row.original.currency || 'USD'
      return (
        <span className="text-ui-fg-base">
          {total ? new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(total / 100) : '-'}
        </span>
      )
    },
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const status = info.getValue()
      return <Badge color={statusColor(status)}>{status || 'pending'}</Badge>
    },
  }),
  col.accessor('paymentStatus', {
    header: 'Payment',
    cell: (info) => {
      const status = info.getValue()
      return status ? <Badge color={statusColor(status)}>{status}</Badge> : <span className="text-ui-fg-muted">-</span>
    },
  }),
  col.accessor('createdAt', {
    header: 'Date',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function OrdersPage() {
  return (
    <DataTableShell<Order>
      kind="order"
      title="Orders"
      description="View and manage customer orders"
      columns={columns}
      detailPath="/orders"
    />
  )
}
