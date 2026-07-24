'use client'

import { useRouter } from 'next/navigation'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { Promotion } from '@/lib/promotions/promotion'

const col = createColumnHelper<Promotion>()

const statusColor = (status: string) =>
  status === 'active' ? 'green' : status === 'inactive' ? 'red' : 'grey'

const columns = [
  col.accessor('code', {
    header: 'Code',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('type', {
    header: 'Type',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (info) => (
      <Badge size="2xsmall" color={statusColor(info.getValue())}>
        {info.getValue() || 'draft'}
      </Badge>
    ),
  }),
  col.accessor('isAutomatic', {
    header: 'Automatic',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ? 'Yes' : 'No'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function PromotionsPage() {
  const router = useRouter()
  return (
    <DataTableShell<Promotion>
      kind="promotion"
      title="Promotions"
      description="Automatic and code-based promotions (v2 engine)"
      columns={columns}
      detailPath="/promotions"
      actions={
        <Button size="small" onClick={() => router.push('/promotions/create')}>
          Create promotion
        </Button>
      }
    />
  )
}
