'use client'

import { useRouter } from 'next/navigation'
import { Text } from '@hanzo/commerce-ui'
import { useOrganizations } from '@hanzo/iam/react'
import { HanzoMark } from '@/components/hanzo-mark'
import { STEP_COUNT, STEPS } from '@/components/onboarding/steps'
import { useOnboarding } from '@/components/onboarding/use-onboarding'
import { dismissOnboarding } from '@/lib/onboarding-state'
import { WizardProgress } from '@/components/onboarding/progress'
import { StoreStep } from '@/components/onboarding/steps/store-step'
import { ProductStep } from '@/components/onboarding/steps/product-step'
import { PaymentsStep } from '@/components/onboarding/steps/payments-step'
import { SubscribeStep } from '@/components/onboarding/steps/subscribe-step'
import { LaunchStep } from '@/components/onboarding/steps/launch-step'
import type { StepProps } from '@/components/onboarding/steps/types'

const STEP_COMPONENTS: Record<(typeof STEPS)[number]['key'], (props: StepProps) => JSX.Element> = {
  store: StoreStep,
  product: ProductStep,
  payments: PaymentsStep,
  subscribe: SubscribeStep,
  launch: LaunchStep,
}

export default function OnboardingPage() {
  const router = useRouter()
  const { currentOrgId } = useOrganizations()
  const { step, ready, goTo, finish } = useOnboarding(STEP_COUNT)

  const isFirst = step === 0
  const isLast = step === STEP_COUNT - 1

  // Leaving the wizard for the dashboard — whether by launching or opting out —
  // marks this org's onboarding done so the dashboard stops routing it back here.
  const leave = () => {
    finish()
    dismissOnboarding(currentOrgId)
    router.push('/overview')
  }

  const advance = () => {
    if (isLast) {
      leave()
      return
    }
    goTo(step + 1)
  }

  const stepProps: StepProps = {
    onNext: advance,
    onBack: isFirst ? undefined : () => goTo(step - 1),
    onSkip: isLast ? undefined : () => goTo(step + 1),
    isFirst,
    isLast,
  }

  const ActiveStep = STEP_COMPONENTS[STEPS[step].key]

  return (
    <div className="mx-auto w-full max-w-2xl">
      <header className="mb-8 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <HanzoMark className="h-7 w-7 text-ui-fg-base" />
          <Text weight="plus" className="text-ui-fg-base">Set up Hanzo Commerce</Text>
        </div>
        <button
          type="button"
          onClick={leave}
          className="text-sm text-ui-fg-muted underline-offset-2 hover:text-ui-fg-base hover:underline"
        >
          Skip for now
        </button>
      </header>

      <div className="mb-8">
        <WizardProgress current={step} onJump={goTo} />
      </div>

      <section className="rounded-2xl border border-ui-border-base bg-ui-bg-subtle p-6 sm:p-8">
        {ready ? <ActiveStep {...stepProps} /> : <div className="h-64 animate-pulse rounded-lg bg-ui-bg-component" />}
      </section>
    </div>
  )
}
