'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { stockLocationDescriptor } from '@/lib/stock-location-descriptor'

export function StockLocationSettingsDetail() {
  return <ResourceEdit descriptor={stockLocationDescriptor} description="Edit this location's name and address." />
}
