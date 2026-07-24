'use client'

import { useRouter } from 'next/navigation'
import { createColumnHelper } from '@tanstack/react-table'
import { Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'

interface Collection {
  id: string
  name: string
  slug: string
  productIds?: string[]
  variantIds?: string[]
  published: boolean
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
  col.accessor((collection) => (collection.productIds?.length ?? 0) + (collection.variantIds?.length ?? 0), {
    id: 'items',
    header: 'Items',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue()}</span>,
  }),
  col.accessor('published', {
    header: 'State',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ? 'Published' : 'Draft'}</span>,
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
  const router = useRouter()
  return (
    <DataTableShell<Collection>
      kind="collection"
      title="Collections"
      description="Organize products into collections"
      columns={columns}
      detailPath="/collections"
      actions={
        <Button size="small" onClick={() => router.push('/collections/create')}>
          Create collection
        </Button>
      }
    />
  )
}
