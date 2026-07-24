'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { inventoryItemDescriptor } from '@/lib/inventory-item'

export default function CreateInventoryItemPage() {
  return <ResourceCreate descriptor={inventoryItemDescriptor} description="Add a stock-keeping unit to track." />
}
