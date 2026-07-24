'use client'

import { createColumnHelper } from '@tanstack/react-table'
import { useRouter } from 'next/navigation'
import { Badge, Button } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'

interface Product {
  id: string
  name: string
  slug: string
  price?: number
  currency: string
  available: boolean
  hidden: boolean
  createdAt: string
}

const col = createColumnHelper<Product>()

const columns = [
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue()}</span>,
  }),
  col.accessor('slug', {
    header: 'Slug',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue()}</span>,
  }),
  col.accessor('price', {
    header: 'Price',
    cell: (info) => {
      const price = info.getValue()
      const currency = info.row.original.currency || 'USD'
      if (price == null) return <span className="text-ui-fg-muted">Not priced</span>
      return (
        <span className="text-ui-fg-base">
          {new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(price / 100)}
        </span>
      )
    },
  }),
  col.accessor((product) => product.hidden ? 'hidden' : product.available ? 'live' : 'draft', {
    id: 'state',
    header: 'Status',
    cell: (info) => {
      const status = info.getValue()
      return (
        <Badge color={status === 'live' ? 'green' : status === 'draft' ? 'grey' : 'orange'}>
          {status}
        </Badge>
      )
    },
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function ProductsPage() {
  const router = useRouter()
  return (
    <DataTableShell<Product>
      kind="product"
      title="Products"
      description="Manage your product catalog"
      columns={columns}
      detailPath="/products"
      actions={
        <Button size="small" onClick={() => router.push('/products/create')}>
          Add product
        </Button>
      }
    />
  )
}
