'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'

interface Customer {
  id: string
  email: string
  firstName: string
  lastName: string
  createdAt: string
  orderCount: number
}

const col = createColumnHelper<Customer>()

const columns = [
  col.accessor('email', {
    header: 'Email',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('firstName', {
    header: 'First Name',
    cell: (info) => <span className="text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('lastName', {
    header: 'Last Name',
    cell: (info) => <span className="text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('orderCount', {
    header: 'Orders',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ?? 0}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Joined',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function CustomersPage() {
  return (
    <DataTableShell<Customer>
      kind="c/user"
      title="Customers"
      description="View and manage customer accounts"
      columns={columns}
      detailPath="/customers"
      actions={
        <>
          <Link href="/customer-groups">
            <Button size="small" variant="secondary">Groups</Button>
          </Link>
          <Link href="/customers/create">
            <Button size="small" variant="primary">Create</Button>
          </Link>
        </>
      }
    />
  )
}
