'use client'

// Credit self-service: top up spendable balance (saved card or a new one) and
// configure auto-recharge. The auto-recharge form composes the ONE shared
// <ResourceForm> engine (react-hook-form + zod) so there is no bespoke form
// plumbing here.

import { useId, useState } from 'react'
import { z } from 'zod'
import { Button, Text, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import { amountToCents, centsToAmount, formatMoney } from '@/lib/format'
import { errorMessage } from '@/lib/forms/schema'
import { useSquareCard } from './use-square-card'
import type { Commerce, PaymentMethod, AutoRecharge } from '@/lib/commerce-client'

const autoRechargeSchema = z.object({
  enabled: z.boolean(),
  thresholdAmount: z.string().trim(),
  topupAmount: z.string().trim(),
})
type AutoRechargeValues = z.infer<typeof autoRechargeSchema>

const autoRechargeFields: FieldSpec<AutoRechargeValues>[] = [
  { name: 'enabled', label: 'Enable auto-recharge', kind: 'switch' },
  { name: 'thresholdAmount', label: 'When balance drops below ($)', placeholder: '10.00' },
  { name: 'topupAmount', label: 'Recharge amount ($)', placeholder: '25.00' },
]

export function CreditPanel({
  client,
  subject,
  methods,
  autoRecharge,
  onChanged,
}: {
  client: Commerce
  subject: string
  methods: PaymentMethod[]
  autoRecharge: AutoRecharge | null
  onChanged: () => void
}) {
  const cardId = useId().replace(/:/g, '')
  const [amount, setAmount] = useState('')
  const [amountError, setAmountError] = useState('')
  const [busy, setBusy] = useState(false)
  const [newCard, setNewCard] = useState(false)
  const [savingAuto, setSavingAuto] = useState(false)
  const square = useSquareCard(client)

  const defaultCard = methods.find((m) => m.isDefault) || methods[0]

  const validAmount = (): number | null => {
    const cents = amountToCents(amount)
    if (!(cents > 0)) {
      setAmountError('Enter an amount greater than 0')
      return null
    }
    setAmountError('')
    return cents
  }

  const topupSaved = async () => {
    const cents = validAmount()
    if (cents == null || !defaultCard) return
    setBusy(true)
    try {
      const res = await client.topup(subject, defaultCard.id, cents)
      toast.success(`Topped up — balance ${formatMoney(res.balanceCents)}`)
      setAmount('')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Top-up failed'))
    } finally {
      setBusy(false)
    }
  }

  const topupNew = async () => {
    const cents = validAmount()
    if (cents == null) return
    setBusy(true)
    try {
      const token = await square.tokenize()
      const res = await client.topupWithToken(token, cents)
      toast.success(`Topped up — balance ${formatMoney(res.balanceCents)}`)
      setAmount('')
      square.reset()
      setNewCard(false)
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Top-up failed'))
    } finally {
      setBusy(false)
    }
  }

  const startNewCard = () => {
    setNewCard(true)
    setTimeout(() => square.mount(cardId), 0)
  }

  const saveAutoRecharge = async (values: AutoRechargeValues) => {
    setSavingAuto(true)
    try {
      await client.setAutoRecharge({
        enabled: values.enabled,
        thresholdCents: amountToCents(values.thresholdAmount || '0'),
        amountCents: amountToCents(values.topupAmount || '0'),
      })
      toast.success('Auto-recharge saved')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not save auto-recharge'))
    } finally {
      setSavingAuto(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <Fieldset title="Add credit" description="Top up your spendable balance.">
        <div className="flex flex-col gap-4 p-5">
          <Field label="Amount ($)" error={amountError} className="max-w-xs">
            <input
              inputMode="decimal"
              placeholder="25.00"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="w-full rounded-md border border-ui-border-base bg-ui-bg-field px-3 py-1.5 text-ui-fg-base outline-none focus:border-ui-border-interactive"
            />
          </Field>

          <div className="flex flex-wrap items-center gap-2">
            {defaultCard && !newCard && (
              <Button size="small" isLoading={busy} onClick={topupSaved}>
                Top up with saved card
              </Button>
            )}
            {!newCard && (
              <Button size="small" variant="secondary" onClick={startNewCard}>
                {defaultCard ? 'Use a new card' : 'Add a card to top up'}
              </Button>
            )}
          </div>

          {newCard && (
            <div className="flex flex-col gap-3 rounded-md border border-ui-border-base p-4">
              <Field label="Card">
                <div id={cardId} className="min-h-16 rounded-md border border-ui-border-base p-3" />
                {square.error && <Text size="small" className="mt-1 text-ui-fg-error">{square.error}</Text>}
              </Field>
              <div className="flex items-center gap-2">
                <Button size="small" disabled={busy || !square.ready} onClick={topupNew}>
                  {busy ? 'Charging…' : 'Charge card'}
                </Button>
                <Button
                  size="small"
                  variant="secondary"
                  disabled={busy}
                  onClick={() => {
                    square.reset()
                    setNewCard(false)
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          )}
        </div>
      </Fieldset>

      <Fieldset
        title="Auto-recharge"
        description="Automatically top up your balance from your default card when it runs low."
      >
        <div className="p-5">
          <ResourceForm
            schema={autoRechargeSchema}
            defaultValues={{
              enabled: autoRecharge?.enabled ?? false,
              thresholdAmount: centsToAmount(autoRecharge?.thresholdCents) || '',
              topupAmount: centsToAmount(autoRecharge?.amountCents) || '',
            }}
            fields={autoRechargeFields}
            onSubmit={saveAutoRecharge}
            submitLabel="Save auto-recharge"
            isPending={savingAuto}
          />
          {!defaultCard && (
            <Text size="small" className="mt-3 text-ui-fg-muted">
              Add a default card above to enable auto-recharge.
            </Text>
          )}
        </div>
      </Fieldset>
    </div>
  )
}
