'use client'

import { Badge, Heading, Text, clx } from '@hanzo/commerce-ui'
import { CheckCircleSolid, CheckMini } from '@hanzo/commerce-icons'
import type { Plan } from '@/lib/commerce-client'

function money(amountCents: number, currency = 'USD') {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency,
    maximumFractionDigits: amountCents % 100 === 0 ? 0 : 2,
  }).format(amountCents / 100)
}

/**
 * Medusa-Cloud-style plan picker: one selectable card per plan. Pure and
 * controlled — the parent owns the selection. Renders skeletons while plans
 * load so the paywall paints immediately with no layout shift.
 */
export function PlanPicker({
  plans,
  loading,
  selectedSlug,
  onSelect,
}: {
  plans: Plan[]
  loading: boolean
  selectedSlug: string
  onSelect: (slug: string) => void
}) {
  if (loading) {
    return (
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {[0, 1].map((i) => (
          <div key={i} className="h-40 animate-pulse rounded-xl bg-ui-bg-component" />
        ))}
      </div>
    )
  }

  if (!plans.length) return null

  return (
    <div className={clx('grid grid-cols-1 gap-3', plans.length > 1 && 'sm:grid-cols-2')}>
      {plans.map((plan) => {
        const active = plan.slug === selectedSlug
        return (
          <button
            key={plan.slug}
            type="button"
            onClick={() => onSelect(plan.slug)}
            aria-pressed={active}
            className={clx(
              'group relative flex flex-col rounded-xl border p-5 text-left transition-colors',
              active
                ? 'border-ui-fg-interactive bg-ui-bg-base ring-1 ring-ui-fg-interactive'
                : 'border-ui-border-base bg-ui-bg-subtle hover:border-ui-border-strong',
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div>
                <Heading level="h3">{plan.name}</Heading>
                <div className="mt-1 flex items-baseline gap-1">
                  <Text size="large" weight="plus" className="text-ui-fg-base">
                    {money(plan.price, plan.currency)}
                  </Text>
                  <Text size="small" className="text-ui-fg-muted">/ {plan.interval}</Text>
                </div>
              </div>
              <span
                className={clx(
                  'mt-1 transition-opacity',
                  active ? 'text-ui-fg-interactive opacity-100' : 'opacity-0',
                )}
              >
                <CheckCircleSolid />
              </span>
            </div>

            {plan.description && (
              <Text size="small" className="mt-2 text-ui-fg-subtle">{plan.description}</Text>
            )}

            {plan.trialPeriodDays ? (
              <Badge color="green" size="2xsmall" className="mt-3 w-fit">
                {plan.trialPeriodDays}-day free trial
              </Badge>
            ) : null}

            {plan.features?.length ? (
              <ul className="mt-4 flex flex-col gap-y-1.5">
                {plan.features.map((feature) => (
                  <li key={feature} className="flex items-center gap-2">
                    <span className="text-ui-fg-muted"><CheckMini /></span>
                    <Text size="small" className="text-ui-fg-subtle">{feature}</Text>
                  </li>
                ))}
              </ul>
            ) : null}
          </button>
        )
      })}
    </div>
  )
}
