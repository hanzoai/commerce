'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { RegionForm } from '@/components/regions/region-form'
import { emptyRegion, toPayload, type Region, type RegionValues } from '@/lib/regions/region'

export default function CreateRegionPage() {
  const router = useRouter()
  const create = useCreate<Region>('region')

  const onSubmit = async (values: RegionValues) => {
    try {
      const created = await create.mutateAsync(toPayload(values))
      toast.success('Region created')
      router.push(created?.id ? `/regions/${created.id}` : '/regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the region')
    }
  }

  return (
    <RegionForm
      title="New region"
      description="Create a market region, then assign the countries it covers."
      submitLabel="Create region"
      defaultValues={emptyRegion}
      submitting={create.isPending}
      onSubmit={onSubmit}
    />
  )
}
