'use client'

import { useRouter } from 'next/navigation'
import { toast } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'
import { CollectionForm } from '@/components/collections/collection-form'
import { emptyCollection, toPayload, type Collection, type CollectionValues } from '@/lib/collections/collection'

export default function CreateCollectionPage() {
  const router = useRouter()
  const create = useCreate<Collection>('collection')

  const onSubmit = async (values: CollectionValues) => {
    try {
      const created = await create.mutateAsync(toPayload(values))
      toast.success('Collection created')
      // Jump straight into the new collection so products can be assigned.
      router.push(created?.id ? `/collections/${created.id}` : '/collections')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not create the collection')
    }
  }

  return (
    <CollectionForm
      title="New collection"
      description="Group products to feature them together in your storefront."
      submitLabel="Create collection"
      defaultValues={emptyCollection}
      submitting={create.isPending}
      autoSlug
      onSubmit={onSubmit}
    />
  )
}
