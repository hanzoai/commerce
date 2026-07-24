'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { inventoryItemDescriptor, type InventoryItem } from '@/lib/inventory-item'
import { InventoryLevels } from './levels-panel'

export function InventoryItemDetail() {
  return (
    <ResourceEdit
      descriptor={inventoryItemDescriptor}
      description="Edit this item and review its stock across locations."
      extra={(record: InventoryItem) => <InventoryLevels id={record.id} />}
    />
  )
}
