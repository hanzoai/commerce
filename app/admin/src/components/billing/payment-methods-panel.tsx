'use client'

// Saved payment methods: list, add a Square-tokenized card, set a default, and
// remove. Every mutation calls the user-scoped /v1/billing/payment-methods
// routes, then asks the page to refresh so the list + balance card re-read.

import { useId, useState } from 'react'
import { Badge, Button, Text, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { ConfirmButton } from '@/components/common/confirm-button'
import { errorMessage } from '@/lib/forms/schema'
import { useSquareCard } from './use-square-card'
import type { Commerce, PaymentMethod } from '@/lib/commerce-client'

function cardLabel(pm: PaymentMethod): string {
  if (pm.card?.brand || pm.card?.last4) {
    const brand = pm.card.brand ? pm.card.brand[0].toUpperCase() + pm.card.brand.slice(1) : 'Card'
    return `${brand} •••• ${pm.card.last4 || '····'}`
  }
  return pm.name || pm.type || 'Payment method'
}

export function PaymentMethodsPanel({
  client,
  subject,
  methods,
  onChanged,
}: {
  client: Commerce
  subject: string
  methods: PaymentMethod[]
  onChanged: () => void
}) {
  const cardId = useId().replace(/:/g, '')
  const [adding, setAdding] = useState(false)
  const [busy, setBusy] = useState(false)
  const square = useSquareCard(client)

  const startAdd = () => {
    setAdding(true)
    // Mount after the container is in the DOM.
    setTimeout(() => square.mount(cardId), 0)
  }

  const cancelAdd = () => {
    square.reset()
    setAdding(false)
  }

  const save = async () => {
    setBusy(true)
    try {
      const token = await square.tokenize()
      await client.addCard(subject, token)
      toast.success('Card added')
      cancelAdd()
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not add card'))
    } finally {
      setBusy(false)
    }
  }

  const makeDefault = async (pm: PaymentMethod) => {
    try {
      await client.setDefaultPaymentMethod(subject, pm.id)
      toast.success('Default payment method updated')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not set default'))
    }
  }

  const remove = async (pm: PaymentMethod) => {
    try {
      await client.removePaymentMethod(pm.id)
      toast.success('Payment method removed')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not remove payment method'))
    }
  }

  return (
    <Fieldset
      title="Payment methods"
      description="Cards on file for subscriptions, top-ups, and auto-recharge."
      actions={
        !adding && (
          <Button size="small" variant="secondary" onClick={startAdd}>
            Add card
          </Button>
        )
      }
    >
      <div className="flex flex-col gap-4 p-5">
        {methods.length === 0 && !adding && (
          <Text size="small" className="text-ui-fg-muted">No payment methods yet.</Text>
        )}

        {methods.map((pm) => (
          <div
            key={pm.id}
            className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-ui-border-base px-4 py-3"
          >
            <div className="flex items-center gap-2">
              <Text size="small" weight="plus">{cardLabel(pm)}</Text>
              {pm.isDefault && <Badge color="green">Default</Badge>}
            </div>
            <div className="flex items-center gap-2">
              {!pm.isDefault && (
                <Button size="small" variant="transparent" onClick={() => makeDefault(pm)}>
                  Make default
                </Button>
              )}
              <ConfirmButton
                onConfirm={() => remove(pm)}
                title="Remove this card?"
                description="Removing this card also detaches it from Square. Any auto-recharge using it will stop."
                confirmText="Remove"
              >
                Remove
              </ConfirmButton>
            </div>
          </div>
        ))}

        {adding && (
          <div className="flex flex-col gap-3 rounded-md border border-ui-border-base p-4">
            <Field label="Card">
              <div id={cardId} className="min-h-16 rounded-md border border-ui-border-base p-3" />
              {square.error && <Text size="small" className="mt-1 text-ui-fg-error">{square.error}</Text>}
            </Field>
            <div className="flex items-center gap-2">
              <Button size="small" disabled={busy || !square.ready} onClick={save}>
                {busy ? 'Saving…' : 'Save card'}
              </Button>
              <Button size="small" variant="secondary" onClick={cancelAdd} disabled={busy}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </div>
    </Fieldset>
  )
}
