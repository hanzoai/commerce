'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { TaxRegionForm } from '@/components/tax-regions/tax-region-form'
import { emptyTaxRegion, toPayload, type TaxRegion, type TaxRegionValues } from '@/lib/tax-regions/tax-region'

export default function CreateTaxRegionPage() {
  const router = useRouter()
  const create = useCreate<TaxRegion>('taxregion')

  const onSubmit = async (values: TaxRegionValues) => {
    try {
      const created = await create.mutateAsync(toPayload(values))
      toast.success('Tax region created')
      router.push(created?.id ? `/tax-regions/${created.id}` : '/tax-regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the tax region')
    }
  }

  return (
    <TaxRegionForm
      title="New tax region"
      description="Create a tax region, then add its rates."
      submitLabel="Create tax region"
      defaultValues={emptyTaxRegion}
      submitting={create.isPending}
      onSubmit={onSubmit}
    />
  )
}
