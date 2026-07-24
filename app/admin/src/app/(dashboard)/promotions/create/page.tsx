'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { PromotionForm } from '@/components/promotions/promotion-form'
import { emptyPromotion, toPayload, type Promotion, type PromotionValues } from '@/lib/promotions/promotion'

export default function CreatePromotionPage() {
  const router = useRouter()
  const create = useCreate<Promotion>('promotion')

  const onSubmit = async (values: PromotionValues) => {
    try {
      const created = await create.mutateAsync(toPayload(values))
      toast.success('Promotion created')
      // Jump into the new promotion so its application method can be configured.
      router.push(created?.id ? `/promotions/${created.id}` : '/promotions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the promotion')
    }
  }

  return (
    <PromotionForm
      title="New promotion"
      description="Create a promotion, then add its application method to set the discount."
      submitLabel="Create promotion"
      defaultValues={emptyPromotion}
      submitting={create.isPending}
      onSubmit={onSubmit}
    />
  )
}
