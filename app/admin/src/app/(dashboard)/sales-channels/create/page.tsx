'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { salesChannelDescriptor } from '@/lib/sales-channel'

export default function CreateSalesChannelPage() {
  return (
    <ResourceCreate
      descriptor={salesChannelDescriptor}
      description="Add a storefront or marketplace where your products are sold."
    />
  )
}
