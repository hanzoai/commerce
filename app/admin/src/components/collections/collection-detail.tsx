'use client'

// Detail + edit view for one collection. Reads its id from the route params (the
// [id] segment), fetches the single record client-side (one request, no
// waterfall), and renders the shared <CollectionForm> pre-filled with the general
// fields plus the product-assignment panel. Save updates, Delete removes with a
// confirm — both toast and return to the list.

import { useParams, useRouter } from 'next/navigation'
import { Container, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useGet, useUpdate, useDelete } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'
import { CollectionForm } from './collection-form'
import { CollectionProducts } from './collection-products'
import { toPayload, toValues, type Collection, type CollectionValues } from '@/lib/collections/collection'

function paramId(value: string | string[] | undefined): string | undefined {
  if (Array.isArray(value)) return value[0]
  return value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Collection" description="Loading…" />
      <div className="p-8">
        <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-md" />
          ))}
        </Container>
      </div>
    </div>
  )
}

export function CollectionDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<Collection>('collection', id)
  const update = useUpdate<Collection>('collection')
  const remove = useDelete('collection')

  if (isLoading || !id) {
    return <LoadingState />
  }

  if (!data) {
    return (
      <div>
        <PageHeader title="Collection" description="This collection could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/collections')}
          >
            Back to collections
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: CollectionValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Collection updated')
      router.push('/collections')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the collection')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Collection deleted')
      router.push('/collections')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the collection')
    }
  }

  return (
    <CollectionForm
      title={data.name || 'Collection'}
      description="Edit this collection's details and the products it features."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      extra={<CollectionProducts collection={data} />}
    />
  )
}
