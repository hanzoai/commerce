'use client'

// Draft-order detail: the line-item builder. The draft is fetched in the
// browser (client-only dynamic route); the line items + running total self-fetch
// in parallel. Adding/removing a line re-reads the projected total; the Complete
// button converts the draft into a real order and routes to it.

import { useParams, useRouter } from 'next/navigation'
import { Badge, Button, Skeleton, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { LineItemBuilder } from '@/components/draft-orders/line-item-builder'
import { useGet } from '@/lib/api/hooks'
import { statusOf, STATUS_COLOR, type DraftOrder } from '@/lib/draft-orders/draft-order'

export function DraftOrderDetail() {
  const router = useRouter()
  const params = useParams<{ id: string }>()
  const id = params?.id
  const { data: draft, isLoading, isError } = useGet<DraftOrder>('draft-order', id)

  if (isLoading) return <DetailSkeleton />

  if (isError || !draft) {
    return (
      <div>
        <PageHeader title="Draft order not found" description="It may have been deleted or completed." />
        <div className="p-8">
          <Button size="small" variant="secondary" onClick={() => router.push('/draft-orders')}>
            Back to draft orders
          </Button>
        </div>
      </div>
    )
  }

  const status = statusOf(draft)
  const currency = draft.currency || 'usd'

  return (
    <div>
      <PageHeader
        title="Draft order"
        description={draft.email || draft.customerId || 'No customer set'}
        actions={
          <div className="flex items-center gap-2">
            <Badge size="2xsmall" color={STATUS_COLOR[status]}>
              {status}
            </Badge>
            <Button size="small" variant="secondary" onClick={() => router.push('/draft-orders')}>
              Back
            </Button>
          </div>
        }
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        {status === 'complete' && draft.orderId && (
          <div className="flex items-center justify-between rounded-lg border border-ui-border-base bg-ui-bg-subtle px-5 py-4">
            <Text size="small" className="text-ui-fg-subtle">
              This draft was completed into an order.
            </Text>
            <Button size="small" variant="secondary" onClick={() => router.push(`/orders/${draft.orderId}`)}>
              View order
            </Button>
          </div>
        )}
        <LineItemBuilder id={draft.id} currency={currency} readOnly={status === 'complete'} />
      </div>
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div>
      <div className="border-b border-ui-border-base px-8 py-6">
        <Skeleton className="h-7 w-56" />
        <Skeleton className="mt-2 h-4 w-32" />
      </div>
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        {[0, 1].map((i) => (
          <div key={i} className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-5">
            <Skeleton className="h-4 w-40" />
            <div className="mt-4 flex flex-col gap-y-3">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        ))}
        <Text size="small" className="sr-only">
          Loading draft order…
        </Text>
      </div>
    </div>
  )
}
