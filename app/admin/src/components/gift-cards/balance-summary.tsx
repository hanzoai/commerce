'use client'

// The live balance card on the gift-card detail page. Reads
// GET /v1/gift-card/:id/balance (a projection: initial − Σ redemptions) in
// parallel with the card fetch — no waterfall. Memoized so redeem/void
// re-renders of siblings don't re-render the stat row until the balance changes.

import { memo } from 'react'
import { Badge, Text } from '@hanzo/commerce-ui'
import { Fieldset } from '@/components/common/field'
import { useResourceActionData } from '@/lib/api/hooks'
import { formatMoney } from '@/lib/format'
import { statusOf, STATUS_COLOR, type GiftCard, type GiftCardBalance } from '@/lib/gift-cards/gift-card'

export const BalanceSummary = memo(function BalanceSummary({ id, card }: { id: string; card: GiftCard }) {
  const { data, isLoading } = useResourceActionData<GiftCardBalance>('gift-card', id, 'balance')

  const currency = data?.currency || card.currency || 'usd'
  const initial = data?.initialBalanceCents ?? card.initialBalanceCents
  const spendable = data?.balanceCents
  const redeemed = spendable != null ? initial - spendable : undefined
  const status = statusOf(card)

  return (
    <Fieldset
      title="Balance"
      description="Live spendable balance projected from the redemption ledger."
      actions={
        <Badge size="2xsmall" color={STATUS_COLOR[status]}>
          {status}
        </Badge>
      }
    >
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        <Stat label="Spendable" value={isLoading || spendable == null ? '—' : formatMoney(spendable, currency)} emphasis />
        <Stat label="Initial value" value={formatMoney(initial, currency)} />
        <Stat label="Redeemed" value={redeemed == null ? '—' : formatMoney(redeemed, currency)} />
      </div>
    </Fieldset>
  )
})

function Stat({ label, value, emphasis }: { label: string; value: string; emphasis?: boolean }) {
  return (
    <div className="flex flex-col gap-y-0.5">
      <Text size="xsmall" className="text-ui-fg-muted">
        {label}
      </Text>
      <Text className={emphasis ? 'text-lg font-semibold text-ui-fg-base' : 'text-ui-fg-base'}>{value}</Text>
    </div>
  )
}
