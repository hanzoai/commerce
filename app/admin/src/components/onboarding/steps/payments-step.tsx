'use client'

import Link from 'next/link'
import { Badge, Text } from '@hanzo/commerce-ui'
import { useIntegrations } from '@/lib/api/hooks'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

const PROVIDERS = [
  { type: 'square', name: 'Square', note: 'Built-in checkout, cards, and in-person payments.' },
  { type: 'stripe', name: 'Stripe', note: 'Global card processing via Stripe Connect.' },
]

export function PaymentsStep({ onNext, onBack, onSkip }: StepProps) {
  const { data: integrations = [] } = useIntegrations()
  const connected = new Set(integrations.filter((i) => i.enabled).map((i) => i.type))

  return (
    <WizardStep
      title="Connect a payment provider"
      description="Accept payments by connecting a provider. Credentials are stored in Hanzo KMS — never in the browser."
    >
      <div className="grid gap-4 sm:grid-cols-2">
        {PROVIDERS.map((provider) => {
          const isOn = connected.has(provider.type)
          return (
            <Link
              key={provider.type}
              href={`/integrations?provider=${provider.type}`}
              className="flex flex-col rounded-xl border border-ui-border-base bg-ui-bg-subtle p-5 transition-colors hover:border-ui-border-strong"
            >
              <div className="flex items-center justify-between">
                <Text weight="plus" className="text-ui-fg-base">{provider.name}</Text>
                {isOn ? <Badge color="green">Connected</Badge> : <Badge color="grey">Connect</Badge>}
              </div>
              <Text size="small" className="mt-2 text-ui-fg-muted">{provider.note}</Text>
            </Link>
          )
        })}
      </div>

      <Text size="small" className="mt-5 text-ui-fg-muted">
        Browse every provider in the{' '}
        <Link href="/integrations" className="underline underline-offset-2 hover:text-ui-fg-base">
          integrations marketplace
        </Link>
        .
      </Text>

      <StepNav
        onNext={onNext}
        onBack={onBack}
        onSkip={onSkip}
        nextLabel="Continue"
        skipLabel="Do this later"
      />
    </WizardStep>
  )
}
