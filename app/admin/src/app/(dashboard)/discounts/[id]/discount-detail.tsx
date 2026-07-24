'use client'

import { useParams, useRouter } from 'next/navigation'
import { Badge, Button, Skeleton, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { DiscountForm } from '@/components/discounts/discount-form'
import { formatDiscountValue, statusColor, useDiscount } from '@/lib/discounts'

export function DiscountDetail() {
  const router = useRouter()
  const params = useParams<{ id: string }>()
  const id = params?.id
  const { data, isLoading, isError } = useDiscount(id)

  if (isLoading) return <DetailSkeleton />

  if (isError || !data) {
    return (
      <div>
        <PageHeader title="Discount not found" description="It may have been deleted." />
        <div className="p-8">
          <Button size="small" variant="secondary" onClick={() => router.push('/discounts')}>
            Back to discounts
          </Button>
        </div>
      </div>
    )
  }

  const status = data.status || 'draft'

  return (
    <div>
      <PageHeader
        title={data.code || 'Discount'}
        description={formatDiscountValue(data)}
        actions={
          <Badge size="2xsmall" color={statusColor(status)}>
            {status}
          </Badge>
        }
      />
      <DiscountForm mode="edit" promotion={data} />
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
        {[0, 1, 2].map((i) => (
          <div key={i} className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-5">
            <Skeleton className="h-4 w-40" />
            <div className="mt-4 flex flex-col gap-y-3">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        ))}
        <Text size="small" className="sr-only">
          Loading discount…
        </Text>
      </div>
    </div>
  )
}
