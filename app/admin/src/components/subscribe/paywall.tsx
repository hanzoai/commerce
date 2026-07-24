'use client'

import { useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Badge, Button, Container, Heading, Input, Text, toast } from '@hanzo/commerce-ui'
import { ArrowRightMini, CheckCircleSolid } from '@hanzo/commerce-icons'
import { HanzoMark } from '@/components/hanzo-mark'
import { PlanPicker } from './plan-picker'
import { FieldRow } from './field-row'
import { useSubscribe } from './use-subscribe'
import type { SquareCardHandle } from './square-card'

// The card widget carries the Square SDK — split it out of the first paint.
const SquareCard = dynamic(() => import('./square-card').then((m) => m.SquareCard), {
  ssr: false,
  loading: () => <div className="min-h-[52px] animate-pulse rounded-lg bg-ui-bg-component" />,
})

const AFTER_SUBSCRIBE = '/overview'

const billingSchema = z.object({
  legalName: z.string().trim().min(1, 'Enter the legal name on the account.'),
  billingEmail: z.string().trim().email('Enter a valid billing email.'),
})
type BillingForm = z.infer<typeof billingSchema>

export function Paywall() {
  const router = useRouter()
  const {
    enabled,
    plans,
    plansLoading,
    subscription,
    subscriptionLoading,
    paymentConfig,
    selectedSlug,
    setSelectedSlug,
    selectedPlan,
    isTrialing,
    isActive,
    subscribe,
    startTrial,
    redeemInvite,
  } = useSubscribe()

  const cardRef = useRef<SquareCardHandle>(null)
  const [inviteCode, setInviteCode] = useState('')

  const form = useForm<BillingForm>({
    resolver: zodResolver(billingSchema),
    defaultValues: { legalName: '', billingEmail: '' },
  })

  const done = () => {
    router.replace(AFTER_SUBSCRIBE)
  }

  // legalName + billingEmail are collected for display/validation only — the
  // backend subscribeCardRequest never persists them, so they don't hit the wire.
  const onSubscribe = form.handleSubmit(async () => {
    if (!paymentConfig) {
      toast.error('Card payments are not configured for this account.')
      return
    }
    try {
      const sourceId = await cardRef.current!.tokenize()
      await subscribe.mutateAsync({
        planSlug: selectedSlug,
        sourceId,
        currency: selectedPlan?.currency ?? 'USD',
      })
      toast.success('Subscription active. Welcome aboard.')
      done()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Subscription failed.')
    }
  })

  const onStartTrial = async () => {
    try {
      await startTrial.mutateAsync()
      toast.success('Your free trial has started.')
      done()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not start the trial.')
    }
  }

  const onRedeem = async () => {
    const code = inviteCode.trim()
    if (!code) return
    try {
      const result = await redeemInvite.mutateAsync(code)
      if (result?.redeemed === false) {
        toast.error('That invite code is not valid.')
        return
      }
      toast.success('Invite redeemed.')
      done()
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not redeem that code.')
    }
  }

  // Already funded — the paywall was reached in error. Offer a way back.
  if (isActive) {
    return (
      <Container className="w-full max-w-lg p-8 text-center">
        <span className="mx-auto flex text-ui-tag-green-icon"><CheckCircleSolid /></span>
        <Heading level="h2" className="mt-4">You're all set</Heading>
        <Text className="mt-2 text-ui-fg-muted">Your subscription is active.</Text>
        <Button className="mt-6" onClick={done}>Go to dashboard</Button>
      </Container>
    )
  }

  const subscribing = subscribe.isPending || form.formState.isSubmitting
  const errors = form.formState.errors

  return (
    <div className="w-full max-w-2xl">
      <div className="mb-8 flex flex-col items-center text-center">
        <HanzoMark className="h-10 w-10 text-ui-fg-base" />
        <Heading level="h1" className="mt-5">Choose your plan</Heading>
        <Text className="mt-2 max-w-md text-ui-fg-muted">
          Products, orders, customers, inventory, integrations, storefront tools, and the
          commerce assistant — everything to build and run your store.
        </Text>
        {isTrialing && subscription?.trialEndsAt && (
          <Badge color="green" className="mt-4">
            Trialing · ends {new Date(subscription.trialEndsAt).toLocaleDateString()}
          </Badge>
        )}
      </div>

      <PlanPicker
        plans={plans}
        loading={plansLoading}
        selectedSlug={selectedSlug}
        onSelect={setSelectedSlug}
      />

      <Container className="mt-6 flex flex-col gap-y-5 p-6">
        <div>
          <Heading level="h3">Billing details</Heading>
          <Text size="small" className="mt-1 text-ui-fg-muted">
            Charged to the card below. Cancel anytime.
          </Text>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FieldRow
            label="Legal name"
            placeholder="Acme, Inc."
            autoComplete="name"
            error={errors.legalName?.message}
            {...form.register('legalName')}
          />
          <FieldRow
            label="Billing email"
            type="email"
            placeholder="billing@acme.com"
            autoComplete="email"
            error={errors.billingEmail?.message}
            {...form.register('billingEmail')}
          />
        </div>

        <div className="flex flex-col gap-y-1.5">
          <Text size="small" weight="plus" className="text-ui-fg-base">Card</Text>
          {enabled && paymentConfig ? (
            <SquareCard ref={cardRef} config={paymentConfig} />
          ) : (
            <div className="rounded-lg border border-ui-border-base bg-ui-bg-field px-3 py-3">
              <Text size="xsmall" className="text-ui-fg-muted">
                {paymentConfig === null
                  ? 'Card payments are not configured for this account.'
                  : 'Preparing secure card field…'}
              </Text>
            </div>
          )}
        </div>

        <Button className="w-full" onClick={onSubscribe} isLoading={subscribing} disabled={!paymentConfig}>
          {selectedPlan
            ? `Subscribe — ${new Intl.NumberFormat('en-US', { style: 'currency', currency: selectedPlan.currency, maximumFractionDigits: selectedPlan.price % 100 === 0 ? 0 : 2 }).format(selectedPlan.price / 100)} / ${selectedPlan.interval}`
            : 'Subscribe'}
        </Button>

        {!isTrialing && selectedPlan?.trialPeriodDays ? (
          <Button
            variant="secondary"
            className="w-full"
            onClick={onStartTrial}
            isLoading={startTrial.isPending}
          >
            Start {selectedPlan.trialPeriodDays}-day free trial
          </Button>
        ) : null}
      </Container>

      <Container className="mt-4 p-6">
        <Heading level="h3">Have an invite code?</Heading>
        <Text size="small" className="mt-1 text-ui-fg-muted">
          Redeem it to unlock access without a card.
        </Text>
        <div className="mt-3 flex flex-col gap-2 sm:flex-row">
          <Input
            placeholder="INVITE-CODE"
            value={inviteCode}
            onChange={(e) => setInviteCode(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') { e.preventDefault(); onRedeem() }
            }}
            className="sm:flex-1"
          />
          <Button
            variant="secondary"
            onClick={onRedeem}
            isLoading={redeemInvite.isPending}
            disabled={!inviteCode.trim()}
          >
            Redeem <ArrowRightMini />
          </Button>
        </div>
      </Container>

      {subscriptionLoading && (
        <Text size="xsmall" className="mt-4 block text-center text-ui-fg-muted">
          Checking your subscription…
        </Text>
      )}
    </div>
  )
}

export default Paywall
