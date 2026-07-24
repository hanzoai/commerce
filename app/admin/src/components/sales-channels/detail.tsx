'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { salesChannelDescriptor } from '@/lib/sales-channel'

export function SalesChannelDetail() {
  return <ResourceEdit descriptor={salesChannelDescriptor} description="Edit this sales channel." />
}
