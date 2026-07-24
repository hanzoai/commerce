'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import type { ProductCategory } from '@/lib/category'

const col = createColumnHelper<ProductCategory>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('handle', {
    header: 'Handle',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('isActive', {
    header: 'Status',
    cell: (info) =>
      info.getValue() === false ? <Badge size="2xsmall">Inactive</Badge> : <Badge size="2xsmall" color="green">Active</Badge>,
  }),
  col.accessor('isInternal', {
    header: 'Visibility',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ? 'Internal' : 'Public'}</span>,
  }),
]

export default function CategoriesPage() {
  return (
    <DataTableShell<ProductCategory>
      kind="product-category"
      title="Categories"
      description="Nestable groupings that organize your product catalog"
      columns={columns}
      detailPath="/categories"
      actions={
        <Link href="/categories/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
