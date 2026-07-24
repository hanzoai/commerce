'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { categoryDescriptor } from '@/lib/category'

export default function CreateCategoryPage() {
  return <ResourceCreate descriptor={categoryDescriptor} description="Add a category to organize your catalog." />
}
