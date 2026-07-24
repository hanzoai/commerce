'use client'

// The redeem panel on the gift-card detail page. Dynamically imported (edit-only,
// off the first paint). A redeem carries a client idempotency key so a
// double-submit debits ONCE (the server dedups on it); the key is regenerated
// after a success so the NEXT redeem is a distinct debit. On success every
// gift-card query (balance + redemptions) is invalidated by the action hook, so
// the balance card and ledger re-read.

import { useRef, useState } from 'react'
import { Button, Input, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { useResourceAction } from '@/lib/api/hooks'
import { amountToCents, formatMoney } from '@/lib/format'
import type { GiftCard, RedeemResult } from '@/lib/gift-cards/gift-card'

function newIdempotencyKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `gc-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function GiftCardActions({ id, card }: { id: string; card: GiftCard }) {
  const [amount, setAmount] = useState('')
  const [orderId, setOrderId] = useState('')
  const [error, setError] = useState('')
  const idemKey = useRef(newIdempotencyKey())
  const redeem = useResourceAction<RedeemResult, {
    amountCents: number
    currency: string
    orderId?: string
    idempotencyKey: string
  }>('gift-card', id, 'redeem')

  const currency = card.currency || 'usd'

  const submit = async () => {
    const cents = amountToCents(amount)
    if (!(cents > 0)) {
      setError('Enter an amount greater than 0')
      return
    }
    setError('')
    try {
      const res = await redeem.mutateAsync({
        amountCents: cents,
        currency,
        orderId: orderId.trim() || undefined,
        idempotencyKey: idemKey.current,
      })
      toast.success(`Redeemed ${formatMoney(cents, currency)} — ${formatMoney(res.balanceCents, currency)} remaining`)
      setAmount('')
      setOrderId('')
      idemKey.current = newIdempotencyKey()
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Redeem failed'
      setError(message)
      toast.error(message)
    }
  }

  return (
    <Fieldset title="Redeem" description="Draw an amount from this card. Idempotent — a double-submit debits once.">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field label="Amount" error={error}>
          <Input
            inputMode="decimal"
            placeholder="0.00"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void submit()
              }
            }}
          />
        </Field>
        <Field label="Order ID" optional hint="Attach this debit to an order.">
          <Input placeholder="order_…" value={orderId} onChange={(e) => setOrderId(e.target.value)} />
        </Field>
      </div>
      <div className="flex justify-end">
        <Button type="button" size="small" onClick={submit} isLoading={redeem.isPending}>
          Redeem
        </Button>
      </div>
    </Fieldset>
  )
}
