'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { Role } from '@/lib/role'

const col = createColumnHelper<Role>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('description', {
    header: 'Description',
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

export default function RolesPage() {
  return (
    <DataTableShell<Role>
      kind="role"
      title="Roles"
      description="Permission groups you can assign to team members"
      columns={columns}
      detailPath="/roles"
      actions={
        <Link href="/roles/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
