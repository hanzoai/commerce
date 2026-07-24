'use client'

import { useState } from 'react'
import { Badge, Input, Label, Select, Text, toast } from '@hanzo/commerce-ui'
import { useStores, useStore } from '@/lib/api/hooks'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

const CURRENCIES = ['usd', 'eur', 'gbp', 'cad', 'aud', 'jpy']

export function StoreStep({ onNext, onSkip, isFirst }: StepProps) {
  const { data: store } = useStore()
  const { create } = useStores()
  const [name, setName] = useState('')
  const [currency, setCurrency] = useState('usd')

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
    try {
      await create.mutateAsync({ name: clean, currency })
      toast.success('Store created', { description: clean })
      onNext()
    } catch {
      toast.error('Could not create store', { description: 'Please try again.' })
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
