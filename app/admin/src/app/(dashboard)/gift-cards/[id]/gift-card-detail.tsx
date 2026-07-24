'use client'

// Gift-card detail + edit. The card is fetched in the browser (client-only
// dynamic route); the balance card and the redemption ledger self-fetch in
// PARALLEL with it (no waterfall). The redeem + redemptions panels are
// dynamically imported so the balance and editable settings paint first.

import dynamic from 'next/dynamic'
import { useParams, useRouter } from 'next/navigation'
import { Badge, Button, Skeleton, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { GiftCardForm } from '@/components/gift-cards/gift-card-form'
import { BalanceSummary } from '@/components/gift-cards/balance-summary'
import { useGet } from '@/lib/api/hooks'
import { statusOf, STATUS_COLOR, type GiftCard } from '@/lib/gift-cards/gift-card'

const GiftCardActions = dynamic(
  () => import('@/components/gift-cards/gift-card-actions').then((m) => m.GiftCardActions),
  { ssr: false, loading: () => <Skeleton className="h-40 w-full" /> },
)
const RedemptionsPanel = dynamic(
  () => import('@/components/gift-cards/redemptions-panel').then((m) => m.RedemptionsPanel),
  { ssr: false, loading: () => <Skeleton className="h-40 w-full" /> },
)

export function GiftCardDetail() {
  const router = useRouter()
  const params = useParams<{ id: string }>()
  const id = params?.id
  const { data: card, isLoading, isError } = useGet<GiftCard>('gift-card', id)

  if (isLoading) return <DetailSkeleton />

  if (isError || !card) {
    return (
      <div>
        <PageHeader title="Gift card not found" description="It may have been deleted." />
        <div className="p-8">
          <Button size="small" variant="secondary" onClick={() => router.push('/gift-cards')}>
            Back to gift cards
          </Button>
        </div>
      </div>
    )
  }

  const status = statusOf(card)
  const currency = card.currency || 'usd'

  return (
    <div>
      <PageHeader
        title={card.code || 'Gift card'}
        description="Prepaid gift card"
        actions={
          <Badge size="2xsmall" color={STATUS_COLOR[status]}>
            {status}
          </Badge>
        }
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        <BalanceSummary id={card.id} card={card} />
        <GiftCardActions id={card.id} card={card} />
        <GiftCardForm mode="edit" card={card} />
        <RedemptionsPanel id={card.id} currency={currency} />
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
          Loading gift card…
        </Text>
      </div>
    </div>
  )
}
