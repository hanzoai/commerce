'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { DataTableShell } from '@/components/common/data-table-shell'

interface Collection {
  id: string
  name: string
  slug: string
  productCount: number
  createdAt: string
}

const col = createColumnHelper<Collection>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('slug', {
    header: 'Slug',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('productCount', {
    header: 'Products',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ?? 0}</span>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function CollectionsPage() {
  return (
    <DataTableShell<Collection>
      kind="collection"
      title="Collections"
      description="Organize products into collections"
      columns={columns}
    />
  )
}
