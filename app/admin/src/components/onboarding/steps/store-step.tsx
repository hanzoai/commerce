'use client'

import { useState } from 'react'
import { Badge, Input, Label, Select, Text, toast } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { useStores, useStore } from '@/lib/api/hooks'
import { Commerce } from '@/lib/commerce-client'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

const CURRENCIES = ['usd', 'eur', 'gbp', 'cad', 'aud', 'jpy']

export function StoreStep({ onNext, onSkip, isFirst }: StepProps) {
  const { data: store } = useStore()
  const { create } = useStores()
  const { accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const [name, setName] = useState('')
  const [currency, setCurrency] = useState('usd')
  const [error, setError] = useState<string | null>(null)

  // Store already exists (returning merchant / came via ?onboarding=1): confirm
  // and move on rather than minting a second store.
  if (store) {
    return (
      <WizardStep title="Create your store" description="Your store is ready — continue setting things up.">
        <div className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-5">
          <div className="flex items-center justify-between gap-3">
            <div>
              <Text weight="plus" className="text-ui-fg-base">{store.name}</Text>
              <Text size="small" className="mt-0.5 text-ui-fg-muted">
                {store.domain ?? store.slug}
                {store.currency ? ` · ${store.currency.toUpperCase()}` : ''}
              </Text>
            </div>
            <Badge color="green">Created</Badge>
          </div>
        </div>
        <StepNav onNext={onNext} hideBack hideSkip nextLabel="Continue" />
      </WizardStep>
    )
  }

  const submit = async () => {
    const clean = name.trim()
    if (!clean) return
    setError(null)
    try {
      const created = await create.mutateAsync({ name: clean, currency })
      // Start the funded trial IMMEDIATELY so the org has store access through the
      // REST of onboarding: the next step reads/creates products, which are paywall-
      // gated — without an active trial that call 402s and ejects the wizard to
      // /subscribe before the merchant ever finishes. Idempotent (no-ops for a
      // returning/comped org) and best-effort (a hiccup here must not block onboarding;
      // the dashboard access gate re-attempts the trial on payment_required).
      const newId =
        (created as { id?: string; store?: { id?: string } })?.id ??
        (created as { store?: { id?: string } })?.store?.id
      if (newId && accessToken) {
        try {
          await new Commerce({ token: accessToken, org: currentOrgId ?? undefined }).startStoreTrial(newId)
        } catch {
          /* best-effort: the dashboard access gate retries on payment_required */
        }
      }
      toast.success('Store created', { description: clean })
      onNext()
    } catch (e) {
      const message = e instanceof Error && e.message ? e.message : 'Please try again.'
      setError(message)
      toast.error('Could not create store', { description: message })
    }
  }

  return (
    <WizardStep
      title="Create your store"
      description="Name your store and pick a default currency. You can change both later in Settings."
    >
      <div className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="store-name" weight="plus">Store name</Label>
          <Input
            id="store-name"
            placeholder="Acme Coffee"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="store-currency" weight="plus">Default currency</Label>
          <Select value={currency} onValueChange={setCurrency}>
            <Select.Trigger id="store-currency">
              <Select.Value />
            </Select.Trigger>
            <Select.Content>
              {CURRENCIES.map((code) => (
                <Select.Item key={code} value={code}>
                  {code.toUpperCase()}
                </Select.Item>
              ))}
            </Select.Content>
          </Select>
        </div>
      </div>
      {error && (
        <div className="mt-5 rounded-lg border border-ui-tag-red-border bg-ui-tag-red-bg px-4 py-3">
          <Text size="small" className="text-ui-tag-red-text">{error}</Text>
        </div>
      )}
      <StepNav
        onNext={submit}
        onSkip={onSkip}
        hideBack={isFirst}
        nextLabel="Create store"
        nextDisabled={!name.trim()}
        nextLoading={create.isPending}
      />
    </WizardStep>
  )
}
