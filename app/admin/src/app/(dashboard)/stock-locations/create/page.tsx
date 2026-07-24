'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { stockLocationDescriptor } from '@/lib/stock-location-descriptor'

export default function CreateStockLocationPage() {
  return (
    <ResourceCreate
      descriptor={stockLocationDescriptor}
      title="Add stock location"
      submitLabel="Create location"
      description="Create a location that holds sellable inventory."
    />
  )
}
