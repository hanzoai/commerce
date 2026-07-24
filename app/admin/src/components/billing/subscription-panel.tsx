'use client'

// Subscription self-management: shows the org's current plan + status + renewal,
// lets an owner change/upgrade/downgrade the plan (from the live GET
// /v1/billing/plans catalog — never a hardcoded slug), and cancel / reactivate.
// When the org has no subscription yet it becomes a subscribe surface: pick a
// plan, enter a card, and POST /v1/billing/subscribe/card.

import { useEffect, useId, useMemo, useState } from 'react'
import { Badge, Button, Select, Text, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { ConfirmButton } from '@/components/common/confirm-button'
import { formatMoney, formatDate } from '@/lib/format'
import { errorMessage } from '@/lib/forms/schema'
import { useSquareCard } from './use-square-card'
import type { Commerce, BillingSubscription, Plan } from '@/lib/commerce-client'

const STATUS_COLOR: Record<string, 'green' | 'orange' | 'red' | 'grey'> = {
  active: 'green',
  trialing: 'orange',
  past_due: 'red',
  unpaid: 'red',
  canceled: 'grey',
}

function planLabel(p: Plan): string {
  const price = p.price ? `${formatMoney(p.price, p.currency)}/${p.interval || 'mo'}` : 'Free'
  return `${p.name} — ${price}`
}

export function SubscriptionPanel({
  client,
  subscription,
  plans,
  onChanged,
}: {
  client: Commerce
  subscription: BillingSubscription | null
  plans: Plan[]
  onChanged: () => void
}) {
  const cardId = useId().replace(/:/g, '')
  const currentSlug = subscription?.planId || ''
  const [selected, setSelected] = useState(currentSlug || plans[0]?.slug || '')
  const [busy, setBusy] = useState(false)
  const square = useSquareCard(client)

  useEffect(() => {
    if (currentSlug) setSelected(currentSlug)
  }, [currentSlug])

  // Only real, purchasable plans (a price > 0 or an explicitly free tier) — skip
  // contact-sales entries which have no self-serve checkout.
  const options = useMemo(() => plans.filter((p) => !p.contactSales && p.slug), [plans])

  const status = (subscription?.status || '').toLowerCase()
  const statusColor = STATUS_COLOR[status] || 'grey'

  const change = async () => {
    if (!subscription || !selected || selected === currentSlug) return
    setBusy(true)
    try {
      await client.changePlan(subscription.id, selected)
      toast.success('Plan updated')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not change plan'))
    } finally {
      setBusy(false)
    }
  }

  const cancel = async () => {
    if (!subscription) return
    try {
      await client.cancelSubscription(subscription.id, true)
      toast.success('Subscription will cancel at period end')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not cancel subscription'))
    }
  }

  const reactivate = async () => {
    if (!subscription) return
    setBusy(true)
    try {
      await client.reactivateSubscription(subscription.id)
      toast.success('Subscription reactivated')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not reactivate subscription'))
    } finally {
      setBusy(false)
    }
  }

  const subscribe = async () => {
    if (!selected) return
    setBusy(true)
    try {
      const token = await square.tokenize()
      await client.subscribeCard({ planSlug: selected, sourceId: token, currency: 'usd' })
      toast.success('Subscription started')
      square.reset()
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not start subscription'))
    } finally {
      setBusy(false)
    }
  }

  // ── No subscription yet → subscribe surface ────────────────────────────────
  if (!subscription) {
    return (
      <Fieldset title="Subscription" description="Choose a plan and add a card to start your subscription.">
        <div className="flex flex-col gap-4 p-5">
          <Field label="Plan">
            <Select value={selected} onValueChange={setSelected}>
              <Select.Trigger>
                <Select.Value placeholder="Select a plan" />
              </Select.Trigger>
              <Select.Content>
                {options.map((p) => (
                  <Select.Item key={p.slug} value={p.slug}>
                    {planLabel(p)}
                  </Select.Item>
                ))}
              </Select.Content>
            </Select>
          </Field>
          <Field label="Card">
            <div id={cardId} className="min-h-16 rounded-md border border-ui-border-base p-3" />
            {square.error && <Text size="small" className="mt-1 text-ui-fg-error">{square.error}</Text>}
          </Field>
          {square.ready ? (
            <Button size="small" disabled={busy || !selected} onClick={subscribe}>
              {busy ? 'Working…' : 'Subscribe'}
            </Button>
          ) : (
            <Button size="small" variant="secondary" disabled={square.mounting} onClick={() => square.mount(cardId)}>
              {square.mounting ? 'Loading…' : 'Add card'}
            </Button>
          )}
        </div>
      </Fieldset>
    )
  }

  // ── Existing subscription → manage ─────────────────────────────────────────
  return (
    <Fieldset title="Subscription" description="Change your plan, or cancel and reactivate your subscription.">
      <div className="flex flex-col gap-5 p-5">
        <div className="flex flex-wrap items-center gap-3">
          <Text weight="plus">{subscription.plan?.name || subscription.planId || 'Plan'}</Text>
          <Badge color={statusColor}>{status || 'unknown'}</Badge>
          {subscription.cancelAtPeriodEnd && <Badge color="orange">Cancels at period end</Badge>}
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Price">
            <Text size="small" className="text-ui-fg-subtle">
              {subscription.plan?.price != null
                ? `${formatMoney(subscription.plan.price, subscription.plan.currency)} / ${subscription.plan.interval || 'mo'}`
                : '—'}
            </Text>
          </Field>
          <Field label={subscription.cancelAtPeriodEnd ? 'Access until' : 'Next renewal'}>
            <Text size="small" className="text-ui-fg-subtle">{formatDate(subscription.currentPeriodEnd)}</Text>
          </Field>
        </div>

        <Field label="Change plan" hint="Upgrade or downgrade — proration is applied automatically.">
          <div className="flex flex-wrap items-center gap-2">
            <div className="min-w-56 flex-1">
              <Select value={selected} onValueChange={setSelected}>
                <Select.Trigger>
                  <Select.Value placeholder="Select a plan" />
                </Select.Trigger>
                <Select.Content>
                  {options.map((p) => (
                    <Select.Item key={p.slug} value={p.slug}>
                      {planLabel(p)}
                    </Select.Item>
                  ))}
                </Select.Content>
              </Select>
            </div>
            <Button size="small" disabled={busy || !selected || selected === currentSlug} onClick={change}>
              {busy ? 'Working…' : 'Change plan'}
            </Button>
          </div>
        </Field>

        <div className="flex items-center gap-2 border-t border-ui-border-base pt-4">
          {subscription.cancelAtPeriodEnd || status === 'canceled' ? (
            <Button size="small" variant="secondary" isLoading={busy} onClick={reactivate}>
              Reactivate
            </Button>
          ) : (
            <ConfirmButton
              onConfirm={cancel}
              title="Cancel subscription?"
              description="Your subscription stays active until the end of the current period, then cancels. You can reactivate any time before then."
              confirmText="Cancel subscription"
              cancelText="Keep subscription"
            >
              Cancel subscription
            </ConfirmButton>
          )}
        </div>
      </div>
    </Fieldset>
  )
}
