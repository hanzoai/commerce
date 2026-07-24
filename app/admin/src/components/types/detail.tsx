'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { productTypeDescriptor } from '@/lib/product-type'

export function TypeDetail() {
  return <ResourceEdit descriptor={productTypeDescriptor} description="Edit this type." />
}
