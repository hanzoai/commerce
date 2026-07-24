'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { ProductType } from '@/lib/product-type'

const col = createColumnHelper<ProductType>()

const columns = [
  col.accessor('value', {
    header: 'Value',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function TypesPage() {
  return (
    <DataTableShell<ProductType>
      kind="product-type"
      title="Types"
      description="Classify products by type for reporting and organization"
      columns={columns}
      detailPath="/types"
      actions={
        <Link href="/types/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
