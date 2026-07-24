'use client'

import { useRouter } from 'next/navigation'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { Region } from '@/lib/regions/region'

const col = createColumnHelper<Region>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('currencyCode', {
    header: 'Currency',
    cell: (info) => <span className="text-ui-fg-muted">{(info.getValue() || '-').toUpperCase()}</span>,
  }),
  col.accessor((region) => region.countries?.length ?? 0, {
    id: 'countries',
    header: 'Countries',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue()}</span>,
  }),
  col.accessor('automaticTaxes', {
    header: 'Auto taxes',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ? 'Yes' : 'No'}</span>,
  }),
]

export default function RegionsPage() {
  const router = useRouter()
  return (
    <DataTableShell<Region>
      kind="region"
      title="Regions"
      description="Market regions, their currency, and the countries they cover"
      columns={columns}
      detailPath="/regions"
      actions={
        <Button size="small" onClick={() => router.push('/regions/create')}>
          Create region
        </Button>
      }
    />
  )
}
