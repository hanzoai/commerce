'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { productTypeDescriptor } from '@/lib/product-type'

export default function CreateTypePage() {
  return <ResourceCreate descriptor={productTypeDescriptor} description="Add a product type." />
}
