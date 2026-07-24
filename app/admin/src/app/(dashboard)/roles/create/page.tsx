'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { roleDescriptor } from '@/lib/role'

export default function CreateRolePage() {
  return <ResourceCreate descriptor={roleDescriptor} description="Define a permission group for your team." />
}
