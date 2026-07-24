'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { StockLocationForm } from '@/components/inventory/stock-location-form'
import {
  emptyStockLocation,
  toPayload,
  type StockLocation,
  type StockLocationValues,
} from '@/lib/inventory/stock-location'

export default function CreateStockLocationPage() {
  const router = useRouter()
  const create = useCreate<StockLocation>('stocklocation')

  const onSubmit = async (values: StockLocationValues) => {
    try {
      await create.mutateAsync(toPayload(values))
      toast.success('Stock location created')
      router.push('/inventory')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the stock location')
    }
  }

  return (
    <StockLocationForm
      title="Add stock location"
      description="Create a location that holds sellable inventory."
      submitLabel="Create location"
      defaultValues={emptyStockLocation}
      submitting={create.isPending}
      onSubmit={onSubmit}
    />
  )
}
