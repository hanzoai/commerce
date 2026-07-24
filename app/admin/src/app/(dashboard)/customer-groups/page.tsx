'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { CustomerGroup } from '@/lib/api/models'

const col = createColumnHelper<CustomerGroup>()

const columns = [
  col.accessor('name', {
    header: 'Name',
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

export default function CustomerGroupsPage() {
  return (
    <DataTableShell<CustomerGroup>
      kind="customergroup"
      title="Customer groups"
      description="Group customers for segments, targeting and pricing"
      columns={columns}
      detailPath="/customer-groups"
      actions={
        <Link href="/customer-groups/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
