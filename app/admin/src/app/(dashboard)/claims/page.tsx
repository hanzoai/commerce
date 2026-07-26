'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { useRouter } from 'next/navigation'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { formatMoney } from '@/lib/format'
import { RESOLUTION_LABEL, STATUS_COLOR, type Claim } from '@/lib/claims/claim'

const col = createColumnHelper<Claim>()

const columns = [
  col.accessor('orderId', {
    header: 'Order',
    cell: (info) => (
      <span className="font-medium text-ui-fg-base">{info.getValue()?.slice(-8) || '—'}</span>
    ),
  }),
  col.accessor('resolution', {
    header: 'Resolution',
    cell: (info) => <span className="text-ui-fg-muted">{RESOLUTION_LABEL[info.getValue()] ?? info.getValue()}</span>,
  }),
  col.accessor('amountCents', {
    header: 'Amount',
    cell: (info) => (
      <span className="text-ui-fg-base">
        {info.getValue() ? formatMoney(info.getValue(), info.row.original.currencyCode) : '—'}
      </span>
    ),
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const status = info.getValue()
      return <Badge color={STATUS_COLOR[status] ?? 'grey'}>{status}</Badge>
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

export default function ClaimsPage() {
  const router = useRouter()
  return (
    <DataTableShell<Claim>
      kind="claim"
      title="Claims"
      description="Resolve damaged, wrong, or missing items with a refund or replacement"
      columns={columns}
      detailPath="/claims"
      actions={
        <Button size="small" onClick={() => router.push('/claims/create')}>
          File claim
        </Button>
      }
    />
  )
}
