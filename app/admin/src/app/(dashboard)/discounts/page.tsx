'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { useRouter } from 'next/navigation'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { formatDiscountValue, statusColor, type Promotion } from '@/lib/discounts'

const col = createColumnHelper<Promotion>()

const columns = [
  col.accessor('code', {
    header: 'Code',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '—'}</span>,
  }),
  col.display({
    id: 'value',
    header: 'Value',
    cell: ({ row }) => <span className="text-ui-fg-base">{formatDiscountValue(row.original)}</span>,
  }),
  col.accessor('type', {
    header: 'Type',
    cell: (info) => <span className="capitalize text-ui-fg-muted">{info.getValue() || 'standard'}</span>,
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (info) => {
      const status = info.getValue() || 'draft'
      return <Badge color={statusColor(status)}>{status}</Badge>
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

export default function DiscountsPage() {
  const router = useRouter()
  return (
    <DataTableShell<Promotion>
      kind="promotion"
      title="Discounts"
      description="Promotion codes and automatic discounts"
      columns={columns}
      detailPath="/discounts"
      actions={
        <Button size="small" onClick={() => router.push('/discounts/create')}>
          Add discount
        </Button>
      }
    />
  )
}
