'use client'

// Detail + edit view for one stock location. Reads its id from the route params
// (the [id] segment), fetches the single record client-side (one request, no
// waterfall), and renders the shared <StockLocationForm> pre-filled. Save updates,
// Delete removes with a confirm — both toast and return to the list.

import { useParams, useRouter } from 'next/navigation'
import { Badge, Container, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useGet, useUpdate, useDelete } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'
import { StockLocationForm } from './stock-location-form'
import {
  toPayload,
  toValues,
  type StockLocation,
  type StockLocationValues,
} from '@/lib/inventory/stock-location'

function paramId(value: string | string[] | undefined): string | undefined {
  if (Array.isArray(value)) return value[0]
  return value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Stock location" description="Loading…" />
      <div className="p-8">
        <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-md" />
          ))}
        </Container>
      </div>
    </div>
  )
}

export function StockLocationDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<StockLocation>('stocklocation', id)
  const update = useUpdate<StockLocation>('stocklocation')
  const remove = useDelete('stocklocation')

  if (isLoading || !id) {
    return <LoadingState />
  }

  if (!data) {
    return (
      <div>
        <PageHeader title="Stock location" description="This location could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/inventory')}
          >
            Back to inventory
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: StockLocationValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Stock location updated')
      router.push('/inventory')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the stock location')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Stock location deleted')
      router.push('/inventory')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the stock location')
    }
  }

  const header = (
    <div className="flex flex-wrap items-center gap-2 border-b border-ui-border-base pb-4">
      <Badge size="2xsmall" className="font-mono">
        {data.id}
      </Badge>
      {data.createdAt && (
        <Text size="xsmall" className="text-ui-fg-muted">
          Added {new Date(data.createdAt).toLocaleDateString()}
        </Text>
      )}
    </div>
  )

  return (
    <StockLocationForm
      title={data.name || 'Stock location'}
      description="Edit this location's name and address."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      header={header}
    />
  )
}
