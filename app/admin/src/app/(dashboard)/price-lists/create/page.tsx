'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { PriceListForm } from '@/components/price-lists/price-list-form'
import { emptyPriceList, toPayload, type PriceList, type PriceListValues } from '@/lib/price-lists/price-list'

export default function CreatePriceListPage() {
  const router = useRouter()
  const create = useCreate<PriceList>('pricelist')

  const onSubmit = async (values: PriceListValues) => {
    try {
      const created = await create.mutateAsync(toPayload(values))
      toast.success('Price list created')
      router.push(created?.id ? `/price-lists/${created.id}` : '/price-lists')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the price list')
    }
  }

  return (
    <PriceListForm
      title="New price list"
      description="Create a price list, then add its prices."
      submitLabel="Create price list"
      defaultValues={emptyPriceList}
      submitting={create.isPending}
      onSubmit={onSubmit}
    />
  )
}
