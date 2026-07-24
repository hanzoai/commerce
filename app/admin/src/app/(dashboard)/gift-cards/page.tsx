'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { useRouter } from 'next/navigation'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { formatMoney } from '@/lib/format'
import { statusOf, STATUS_COLOR, type GiftCard } from '@/lib/gift-cards/gift-card'

const col = createColumnHelper<GiftCard>()

const columns = [
  col.accessor('code', {
    header: 'Code',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue()}</span>,
  }),
  col.accessor('initialBalanceCents', {
    header: 'Initial value',
    cell: (info) => (
      <span className="text-ui-fg-base">{formatMoney(info.getValue(), info.row.original.currency)}</span>
    ),
  }),
  col.accessor('currency', {
    header: 'Currency',
    cell: (info) => <span className="uppercase text-ui-fg-muted">{info.getValue()}</span>,
  }),
  col.accessor((card) => statusOf(card), {
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

export default function GiftCardsPage() {
  const router = useRouter()
  return (
    <DataTableShell<GiftCard>
      kind="gift-card"
      title="Gift cards"
      description="Issue and manage prepaid gift cards"
      columns={columns}
      detailPath="/gift-cards"
      actions={
        <Button size="small" onClick={() => router.push('/gift-cards/create')}>
          Add gift card
        </Button>
      }
    />
  )
}
