'use client'

import { PageHeader } from '@/components/common/page-header'
import { ProductForm } from '@/components/products/product-form'
import { useCanWriteProduct } from '@/lib/iam/can'

export default function CreateProductPage() {
  const canWrite = useCanWriteProduct()
  return (
    <div>
      <PageHeader title="New product" description="Add a product to your catalog." />
      <ProductForm mode="create" canWrite={canWrite} />
    </div>
  )
}
