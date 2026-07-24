'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { roleDescriptor } from '@/lib/role'

export function RoleDetail() {
  return <ResourceEdit descriptor={roleDescriptor} description="Edit this role." />
}
