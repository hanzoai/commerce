'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { SalesChannel } from '@/lib/sales-channel'

const col = createColumnHelper<SalesChannel>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('description', {
    header: 'Description',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('isDisabled', {
    header: 'Status',
    cell: (info) =>
      info.getValue() ? <Badge size="2xsmall">Disabled</Badge> : <Badge size="2xsmall" color="green">Enabled</Badge>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function SalesChannelsPage() {
  return (
    <DataTableShell<SalesChannel>
      kind="saleschannel"
      title="Sales channels"
      description="The storefronts and marketplaces where your products are sold"
      columns={columns}
      detailPath="/sales-channels"
      actions={
        <Link href="/sales-channels/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
