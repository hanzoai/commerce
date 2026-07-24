'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { Commerce, type Plan, type Subscription, type PaymentConfig, type SubscribeCardInput } from '@/lib/commerce-client'

/** The default plan the paywall pre-selects — the $20 "pro" tier. */
export const DEFAULT_PLAN_SLUG = 'pro'

const TRIALING = new Set(['trialing', 'trial'])

/**
 * One place that owns every read + write the paywall needs. The three reads
 * (plans · subscription · payment-config) fire in parallel through react-query
 * — no waterfall — and each write invalidates the subscription so the view
 * reflects the new state immediately. Org-scoped query keys keep the cache
 * clean across org switches.
 */
export function useSubscribe() {
  const { accessToken, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()

  const client = useMemo(
    () => new Commerce({ token: accessToken, org: currentOrgId }),
    [accessToken, currentOrgId],
  )

  const enabled = isAuthenticated && !!accessToken
  const key = (name: string) => [currentOrgId ?? '__no_org__', 'subscribe', name]

  const plans = useQuery({ queryKey: key('plans'), queryFn: () => client.getPlans(), enabled })
  const subscription = useQuery({ queryKey: key('subscription'), queryFn: () => client.getSubscription(), enabled })
  const paymentConfig = useQuery({ queryKey: key('payment-config'), queryFn: () => client.getPaymentConfig(), enabled })

  const [selectedSlug, setSelectedSlug] = useState(DEFAULT_PLAN_SLUG)

  // Once plans arrive, keep the selection valid: prefer "pro", else the first.
  useEffect(() => {
    const list = plans.data
    if (!list?.length) return
    if (list.some((p) => p.slug === selectedSlug)) return
    setSelectedSlug(list.find((p) => p.slug === DEFAULT_PLAN_SLUG)?.slug ?? list[0].slug)
  }, [plans.data, selectedSlug])

  const invalidate = () => qc.invalidateQueries({ queryKey: key('subscription') })

  // ONE stable idempotency key per checkout attempt. Every retry / double-submit
  // reuses it so the backend dedups to a single vaulted card + charge + sub; a
  // fresh attempt (after a success) rotates to a new key.
  const idempotencyKey = useRef(crypto.randomUUID())

  const subscribe = useMutation({
    mutationFn: (input: SubscribeCardInput) => client.subscribeCard(input, idempotencyKey.current),
    onSuccess: () => {
      idempotencyKey.current = crypto.randomUUID()
      invalidate()
    },
  })
  const startTrial = useMutation({
    mutationFn: () => client.startTrial(),
    onSuccess: invalidate,
  })
  const redeemInvite = useMutation({
    mutationFn: (code: string) => client.redeemInvite(code),
    onSuccess: invalidate,
  })

  const selectedPlan: Plan | undefined = plans.data?.find((p) => p.slug === selectedSlug)
  const status = subscription.data?.status
  const isTrialing = !!status && TRIALING.has(status)
  const isActive = status === 'active'

  return {
    client,
    enabled,
    plans: plans.data ?? [],
    plansLoading: plans.isLoading,
    subscription: subscription.data as Subscription | null | undefined,
    subscriptionLoading: subscription.isLoading,
    paymentConfig: paymentConfig.data as PaymentConfig | null | undefined,
    selectedSlug,
    setSelectedSlug,
    selectedPlan,
    isTrialing,
    isActive,
    subscribe,
    startTrial,
    redeemInvite,
  }
}
