'use client'

import { useRouter } from 'next/navigation'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { PricePreferencesPanel } from '@/components/price-lists/price-preferences-panel'
import type { PriceList } from '@/lib/price-lists/price-list'

const col = createColumnHelper<PriceList>()

const columns = [
  col.accessor('title', {
    header: 'Title',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('type', {
    header: 'Type',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('status', {
    header: 'Status',
    cell: (info) => (
      <Badge size="2xsmall" color={info.getValue() === 'active' ? 'green' : 'grey'}>
        {info.getValue() || 'draft'}
      </Badge>
    ),
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function PriceListsPage() {
  const router = useRouter()
  return (
    <div>
      <DataTableShell<PriceList>
        kind="pricelist"
        title="Price lists"
        description="Sale and override pricing, plus store-wide price preferences"
        columns={columns}
        detailPath="/price-lists"
        actions={
          <Button size="small" onClick={() => router.push('/price-lists/create')}>
            Create price list
          </Button>
        }
      />
      <div className="px-8 pb-8">
        <PricePreferencesPanel />
      </div>
    </div>
  )
}
