'use client'

import { Badge, Text } from '@hanzo/commerce-ui'
import { useStore, useCount, useIntegrations } from '@/lib/api/hooks'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

export function LaunchStep({ onNext, onBack }: StepProps) {
  const { data: store } = useStore()
  const { data: productCount = 0 } = useCount('product')
  const { data: integrations = [] } = useIntegrations()
  const providers = integrations.filter((i) => i.enabled).length

  const rows: { label: string; value: string; done: boolean }[] = [
    { label: 'Store', value: store?.name ?? 'Not created', done: !!store },
    { label: 'Products', value: `${productCount}`, done: productCount > 0 },
    { label: 'Payment providers', value: `${providers} connected`, done: providers > 0 },
  ]

  return (
    <WizardStep
      title="You're ready to launch"
      description="Here's what you set up. You can complete any remaining steps anytime from your dashboard."
    >
      <div className="divide-y divide-ui-border-base rounded-xl border border-ui-border-base bg-ui-bg-subtle">
        {rows.map((row) => (
          <div key={row.label} className="flex items-center justify-between gap-3 px-5 py-4">
            <div>
              <Text size="small" weight="plus" className="text-ui-fg-base">{row.label}</Text>
              <Text size="small" className="text-ui-fg-muted">{row.value}</Text>
            </div>
            <Badge color={row.done ? 'green' : 'grey'}>{row.done ? 'Done' : 'Pending'}</Badge>
          </div>
        ))}
      </div>

      <StepNav onNext={onNext} onBack={onBack} hideSkip nextLabel="Go to dashboard" />
    </WizardStep>
  )
}
