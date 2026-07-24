'use client'

import { Component, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Commerce } from '@/lib/commerce-client'
import type {
  BalanceDetail,
  Balance,
  BillingSubscription,
  Invoice,
  PaymentMethod,
  AutoRecharge,
  Plan,
  Tier,
} from '@/lib/commerce-client'
import { Text } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { PageHeader } from '@/components/common/page-header'
import { StatCard } from '@/components/common/stat-card'
import { ToasterMount } from '@/components/common/toaster-mount'
import { formatMoney, formatDate } from '@/lib/format'
import { SubscriptionPanel } from '@/components/billing/subscription-panel'
import { PaymentMethodsPanel } from '@/components/billing/payment-methods-panel'
import { InvoicesPanel } from '@/components/billing/invoices-panel'
import { CreditPanel } from '@/components/billing/credit-panel'

// Error boundary to catch useOrganizations or other render errors.
class BillingErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) {
    return { error }
  }
  render() {
    if (this.state.error) {
      return (
        <div>
          <PageHeader title="Billing" description="Plan, balance, payment methods, and invoices" />
          <div className="p-8">
            <Text size="small" className="text-ui-fg-muted">
              Unable to load billing data. Please try refreshing the page.
            </Text>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

export default function BillingPage() {
  return (
    <BillingErrorBoundary>
      <ToasterMount />
      <BillingContent />
    </BillingErrorBoundary>
  )
}

interface Snapshot {
  balance: BalanceDetail | null
  creditBalance: Balance | null
  plans: Plan[]
  subscriptions: BillingSubscription[]
  methods: PaymentMethod[]
  invoices: Invoice[]
  autoRecharge: AutoRecharge | null
  tier: Tier | null
}

const EMPTY: Snapshot = {
  balance: null,
  creditBalance: null,
  plans: [],
  subscriptions: [],
  methods: [],
  invoices: [],
  autoRecharge: null,
  tier: null,
}

// The primary account subscription: the non-bundle row (bundle children ride a
// parent), preferring an active/trialing one.
function primarySubscription(subs: BillingSubscription[]): BillingSubscription | null {
  const real = subs.filter((s) => s.providerType !== 'bundle')
  const live = real.find((s) => {
    const st = String(s.status || '').toLowerCase()
    return st === 'active' || st === 'trialing' || st === 'past_due'
  })
  return live || real[0] || null
}

function BillingContent() {
  const { accessToken: token, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  const [snapshot, setSnapshot] = useState<Snapshot>(EMPTY)
  const [loading, setLoading] = useState(true)
  const [refreshKey, setRefreshKey] = useState(0)

  // The billing subject the backend keys per-org money to (orgBillingKey =
  // org.Name = the X-Org-Id slug). Used as the payment-method customerId and the
  // top-up subject so created cards land where the list + charges read them.
  const subject = useMemo(() => String(currentOrgId ?? '').toLowerCase(), [currentOrgId])

  const client = useMemo(
    () => (token ? new Commerce({ token, org: currentOrgId ?? undefined }) : null),
    [token, currentOrgId],
  )

  const refresh = useCallback(() => setRefreshKey((k) => k + 1), [])

  useEffect(() => {
    if (!isAuthenticated || !client) {
      setLoading(false)
      return
    }
    let alive = true
    setLoading(true)

    Promise.allSettled([
      client.getBalance('me'),
      client.getCreditBalance('me'),
      client.getPlans(),
      client.listSubscriptions(),
      subject ? client.listPaymentMethods(subject) : Promise.resolve([] as PaymentMethod[]),
      client.getInvoices('me', { limit: 50 }),
      client.getAutoRecharge(),
      client.getTier(subject || 'me'),
    ]).then((results) => {
      if (!alive) return
      const [bal, credit, plans, subs, methods, invoices, auto, tier] = results
      setSnapshot({
        balance: bal.status === 'fulfilled' ? (bal.value as BalanceDetail | null) : null,
        creditBalance: credit.status === 'fulfilled' ? credit.value : null,
        plans: plans.status === 'fulfilled' ? plans.value : [],
        subscriptions: subs.status === 'fulfilled' ? subs.value : [],
        methods: methods.status === 'fulfilled' ? methods.value : [],
        invoices: invoices.status === 'fulfilled' ? invoices.value : [],
        autoRecharge: auto.status === 'fulfilled' ? auto.value : null,
        tier: tier.status === 'fulfilled' ? tier.value : null,
      })
      setLoading(false)
    })

    return () => {
      alive = false
    }
  }, [client, isAuthenticated, subject, refreshKey])

  const subscription = useMemo(() => primarySubscription(snapshot.subscriptions), [snapshot.subscriptions])

  const available = snapshot.balance?.available ?? 0
  const credits = snapshot.balance?.creditsRemaining ?? snapshot.creditBalance?.available ?? 0
  const planName =
    subscription?.plan?.name ||
    snapshot.tier?.tier?.displayName ||
    snapshot.tier?.tier?.name ||
    'No plan'
  const renewal = subscription?.currentPeriodEnd

  return (
    <div>
      <PageHeader title="Billing" description="Plan, balance, payment methods, and invoices" />
      <div className="flex flex-col gap-8 p-8">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard label="Balance" value={formatMoney(available)} loading={loading} />
          <StatCard label="Credit" value={formatMoney(credits)} loading={loading} />
          <StatCard label="Plan" value={planName} loading={loading} />
          <StatCard label="Next renewal" value={renewal ? formatDate(renewal) : '—'} loading={loading} />
        </div>

        {client && (
          <>
            <SubscriptionPanel
              client={client}
              subscription={subscription}
              plans={snapshot.plans}
              onChanged={refresh}
            />
            <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
              <PaymentMethodsPanel
                client={client}
                subject={subject}
                methods={snapshot.methods}
                onChanged={refresh}
              />
              <CreditPanel
                client={client}
                subject={subject}
                methods={snapshot.methods}
                autoRecharge={snapshot.autoRecharge}
                onChanged={refresh}
              />
            </div>
            <InvoicesPanel client={client} invoices={snapshot.invoices} onChanged={refresh} />
          </>
        )}
      </div>
    </div>
  )
}
