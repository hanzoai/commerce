'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Badge, Button, Text, toast } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Commerce, type StoreAccess, type Subscription } from '@/lib/commerce-client'
import { useStore } from '@/lib/api/hooks'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

type TrialState =
  | { kind: 'loading' }
  | { kind: 'active'; label: string }
  | { kind: 'trialing'; endsAt?: string }
  | { kind: 'none' }

function describe(access: StoreAccess | null, sub: Subscription | null): TrialState {
  if (access?.status === 'active' || sub?.status === 'active') return { kind: 'active', label: 'Subscribed' }
  if (access?.status === 'trial' || sub?.status === 'trialing') return { kind: 'trialing', endsAt: sub?.trialEndsAt }
  return { kind: 'none' }
}

export function SubscribeStep({ onNext, onBack, onSkip }: StepProps) {
  const { accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const { data: store } = useStore()
  const [state, setState] = useState<TrialState>({ kind: 'loading' })
  const [starting, setStarting] = useState(false)

  const load = async () => {
    if (!accessToken || !currentOrgId) return
    const client = new Commerce({ token: accessToken, org: currentOrgId })
    const [access, sub] = await Promise.all([client.getStoreAccess(store?.id), client.getSubscription()])
    setState(describe(access, sub))
  }

  useEffect(() => {
    let alive = true
    ;(async () => {
      if (!accessToken || !currentOrgId) return
      const client = new Commerce({ token: accessToken, org: currentOrgId })
      const [access, sub] = await Promise.all([client.getStoreAccess(store?.id), client.getSubscription()])
      if (alive) setState(describe(access, sub))
    })()
    return () => {
      alive = false
    }
  }, [accessToken, currentOrgId, store?.id])

  const startTrial = async () => {
    if (!accessToken || !currentOrgId) return
    setStarting(true)
    try {
      const client = new Commerce({ token: accessToken, org: currentOrgId })
      await client.startTrial()
      toast.success('Trial started', { description: 'Your 7-day store trial is active.' })
      await load()
    } catch {
      toast.error('Could not start trial', { description: 'Add a card from billing instead.' })
    } finally {
      setStarting(false)
    }
  }

  return (
    <WizardStep
      title="Start your trial"
      description="Each store is $20 / month on the pro plan and includes a one-time 7-day free trial."
    >
      <div className="rounded-xl border border-ui-border-base bg-ui-bg-subtle p-6">
        {state.kind === 'loading' ? (
          <div className="h-16 animate-pulse rounded-lg bg-ui-bg-component" />
        ) : state.kind === 'active' ? (
          <div className="flex items-center justify-between gap-3">
            <div>
              <Text weight="plus" className="text-ui-fg-base">You are subscribed</Text>
              <Text size="small" className="mt-1 text-ui-fg-muted">Your store is fully active.</Text>
            </div>
            <Badge color="green">Active</Badge>
          </div>
        ) : state.kind === 'trialing' ? (
          <div className="flex items-center justify-between gap-3">
            <div>
              <Text weight="plus" className="text-ui-fg-base">Trial active</Text>
              <Text size="small" className="mt-1 text-ui-fg-muted">
                {state.endsAt ? `Ends ${new Date(state.endsAt).toLocaleDateString()}.` : 'Your 7-day trial is running.'}
                {' '}Add a card before it ends to keep your store live.
              </Text>
            </div>
            <Badge color="blue">Trialing</Badge>
          </div>
        ) : (
          <div className="space-y-4">
            <Text size="small" className="text-ui-fg-muted">
              Start your free trial now, or add a card to subscribe right away.
            </Text>
            <div className="flex flex-wrap items-center gap-3">
              <Button variant="primary" size="small" onClick={startTrial} isLoading={starting}>
                Start free trial
              </Button>
              <Link
                href="/subscribe"
                className="inline-flex items-center rounded-md border border-ui-border-base px-4 py-1.5 text-sm text-ui-fg-base hover:bg-ui-bg-base-hover"
              >
                Add a card
              </Link>
            </div>
          </div>
        )}
      </div>

      <StepNav onNext={onNext} onBack={onBack} onSkip={onSkip} nextLabel="Continue" skipLabel="Decide later" />
    </WizardStep>
  )
}
