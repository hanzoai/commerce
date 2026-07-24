'use client'

import dynamic from 'next/dynamic'
import { useParams, useRouter } from 'next/navigation'
import { Badge, Button, Skeleton, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ProductForm } from '@/components/products/product-form'
import { useProduct } from '@/lib/api/hooks'
import { useCanWriteProduct } from '@/lib/iam/can'
import { statusOf, STATUS_COLOR, type Product } from '@/lib/products/product'

// Collection assignment is edit-only and hits its own endpoint — load it lazily
// so the product fields paint first.
const CollectionsPanel = dynamic(
  () => import('@/components/products/collections-panel').then((m) => m.CollectionsPanel),
  { ssr: false },
)

export function ProductDetail() {
  const router = useRouter()
  const params = useParams<{ id: string }>()
  const id = params?.id
  const canWrite = useCanWriteProduct()
  const { data: product, isLoading, isError } = useProduct(id) as {
    data: Product | undefined
    isLoading: boolean
    isError: boolean
  }

  if (isLoading) return <DetailSkeleton />

  if (isError || !product) {
    return (
      <div>
        <PageHeader title="Product not found" description="It may have been deleted." />
        <div className="p-8">
          <Button size="small" variant="secondary" onClick={() => router.push('/products')}>
            Back to products
          </Button>
        </div>
      </div>
    )
  }

  const status = statusOf(product)

  return (
    <div>
      <PageHeader
        title={product.name || product.slug || 'Product'}
        description={product.slug ? `/${product.slug}` : undefined}
        actions={<Badge size="2xsmall" color={STATUS_COLOR[status]}>{status}</Badge>}
      />
      <ProductForm
        mode="edit"
        product={product}
        canWrite={canWrite}
        extraSections={<CollectionsPanel productId={product.id} disabled={!canWrite} />}
      />
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div>
      <div className="border-b border-ui-border-base px-8 py-6">
        <Skeleton className="h-7 w-56" />
        <Skeleton className="mt-2 h-4 w-32" />
      </div>
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        {[0, 1, 2].map((i) => (
          <div key={i} className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-5">
            <Skeleton className="h-4 w-40" />
            <div className="mt-4 flex flex-col gap-y-3">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        ))}
        <Text size="small" className="sr-only">Loading product…</Text>
      </div>
    </div>
  )
}
