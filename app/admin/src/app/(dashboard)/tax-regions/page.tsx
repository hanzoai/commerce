'use client'

import { useRouter } from 'next/navigation'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { TaxRegion } from '@/lib/tax-regions/tax-region'

const col = createColumnHelper<TaxRegion>()

const columns = [
  col.accessor('countryCode', {
    header: 'Country',
    cell: (info) => <span className="font-medium text-ui-fg-base">{(info.getValue() || '-').toUpperCase()}</span>,
  }),
  col.accessor('provinceCode', {
    header: 'Province',
    cell: (info) => <span className="text-ui-fg-muted">{(info.getValue() || '-').toUpperCase()}</span>,
  }),
  col.accessor('providerId', {
    header: 'Provider',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function TaxRegionsPage() {
  const router = useRouter()
  return (
    <DataTableShell<TaxRegion>
      kind="taxregion"
      title="Tax regions"
      description="Configure tax regions and their rates"
      columns={columns}
      detailPath="/tax-regions"
      actions={
        <Button size="small" onClick={() => router.push('/tax-regions/create')}>
          Create tax region
        </Button>
      }
    />
  )
}
