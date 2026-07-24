'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { categoryDescriptor, type ProductCategory } from '@/lib/category'
import { CategoryChildren } from './children-panel'

export function CategoryDetail() {
  return (
    <ResourceEdit
      descriptor={categoryDescriptor}
      description="Edit this category and browse its subcategories."
      extra={(record: ProductCategory) => <CategoryChildren id={record.id} />}
    />
  )
}
