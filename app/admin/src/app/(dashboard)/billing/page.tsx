'use client'

import { Component, useEffect, useRef, useState, type ReactNode } from 'react'
import { Commerce } from '@/lib/commerce-client'
import { Button, Heading, Text, Container, Badge } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { PageHeader } from '@/components/common/page-header'
import { StatCard } from '@/components/common/stat-card'
import { useStore } from '@/lib/api/hooks'

// Error boundary to catch useOrganizations or other render errors
class BillingErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  render() {
    if (this.state.error) {
      return (
        <div>
          <PageHeader title="Billing" description="Account balance, credits, and invoices" />
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
      <BillingContent />
    </BillingErrorBoundary>
  )
}

function BillingContent() {
  const { accessToken: token, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  const { data: store } = useStore()
  const [balance, setBalance] = useState<any>(null)
  const [creditBalance, setCreditBalance] = useState<any>(null)
  const [invoices, setInvoices] = useState<any[]>([])
  const [tier, setTier] = useState<any>(null)
  const [plan, setPlan] = useState<any>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!isAuthenticated || !token) {
      setLoading(false)
      return
    }

    const client = new Commerce({ token, org: currentOrgId ?? undefined })

    Promise.allSettled([
      client.getBalance('me'),
      client.getCreditBalance('me'),
      client.getInvoices('me', { limit: 10 }),
      client.getTier(currentOrgId || 'me'),
      client.getPlans(),
    ]).then(([balRes, creditRes, invRes, tierRes, plansRes]) => {
      if (balRes.status === 'fulfilled') setBalance(balRes.value)
      if (creditRes.status === 'fulfilled') setCreditBalance(creditRes.value)
      if (invRes.status === 'fulfilled') setInvoices(invRes.value)
      if (tierRes.status === 'fulfilled') setTier(tierRes.value)
      if (plansRes.status === 'fulfilled') setPlan(plansRes.value.find((item) => item.slug === 'pro'))
      setLoading(false)
    })
  }, [token, isAuthenticated, currentOrgId])

  return (
    <div>
      <PageHeader title="Billing" description="Account balance, credits, and invoices" />
      <div className="p-8">
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-3">
          <StatCard
            label="Account Balance"
            value={
              balance
                ? new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(
                    (balance.available ?? 0) / 100
                  )
                : '$0.00'
            }
            loading={loading}
          />
          <StatCard
            label="Credit Balance"
            value={
              creditBalance
                ? new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(
                    (creditBalance.available ?? 0) / 100
                  )
                : '$0.00'
            }
            loading={loading}
          />
          <StatCard label="Invoices" value={invoices.length} loading={loading} />
        </div>

        <Container className="mb-6 p-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-2">
                <Heading level="h3">{plan?.name || 'Pro'}</Heading>
                <Badge color={tier?.tier?.name === 'free' ? 'grey' : 'green'}>
                  {tier?.tier?.displayName || tier?.tier?.name || 'Loading'}
                </Badge>
              </div>
              <Text size="small" className="mt-2 max-w-xl text-ui-fg-muted">
                {plan?.description || 'Everything you need to build and run one store.'}
              </Text>
              <Text size="small" weight="plus" className="mt-3 text-ui-fg-base">
                ${((plan?.price ?? 2000) / 100).toFixed(0)} per store / month
              </Text>
              <Text size="xsmall" className="mt-1 text-ui-fg-muted">
                New stores include a 7-day trial. Adding a card extends it to 30 days.
              </Text>
            </div>
            {token && currentOrgId && store && (
              <Subscribe client={new Commerce({ token, org: currentOrgId })} storeId={store.id} />
            )}
          </div>
        </Container>

        <Container className="p-6">
          <Heading level="h3" className="mb-4">Recent Invoices</Heading>
          {loading ? (
            <div className="space-y-3">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
              ))}
            </div>
          ) : invoices.length === 0 ? (
            <Text size="small" className="py-8 text-center text-ui-fg-muted">No invoices yet</Text>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="border-b border-ui-border-base text-left">
                  <th className="pb-2"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Invoice</Text></th>
                  <th className="pb-2"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Date</Text></th>
                  <th className="pb-2"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Status</Text></th>
                  <th className="pb-2 text-right"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Amount</Text></th>
                </tr>
              </thead>
              <tbody>
                {invoices.map((inv: any) => (
                  <tr key={inv.id} className="border-b border-ui-border-base last:border-0">
                    <td className="py-3"><Text as="span" size="small">{inv.number || inv.id?.slice(-8)}</Text></td>
                    <td className="py-3"><Text as="span" size="small" className="text-ui-fg-muted">{inv.createdAt ? new Date(inv.createdAt).toLocaleDateString() : '-'}</Text></td>
                    <td className="py-3"><Text as="span" size="small" className="text-ui-fg-muted">{inv.status || '-'}</Text></td>
                    <td className="py-3 text-right">
                      <Text as="span" size="small">
                        {inv.total != null
                          ? new Intl.NumberFormat('en-US', {
                              style: 'currency',
                              currency: inv.currency || 'USD',
                            }).format(inv.total / 100)
                          : '-'}
                      </Text>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Container>
      </div>
    </div>
  )
}

function Subscribe({ client, storeId }: { client: Commerce; storeId: string }) {
  const [card, setCard] = useState<any>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const attempt = useRef(crypto.randomUUID())

  const load = async () => {
    setBusy(true)
    setError('')
    try {
      const config = await client.getPaymentConfig()
      if (!config?.applicationId || !config.locationId) throw new Error('Card payments are not configured.')
      if (!(window as any).Square) {
        await new Promise<void>((resolve, reject) => {
          const script = document.createElement('script')
          script.src = config.environment === 'sandbox'
            ? 'https://sandbox.web.squarecdn.com/v1/square.js'
            : 'https://web.squarecdn.com/v1/square.js'
          script.onload = () => resolve()
          script.onerror = () => reject(new Error('Could not load card payments.'))
          document.head.appendChild(script)
        })
      }
      const payments = (window as any).Square.payments(config.applicationId, config.locationId)
      const nextCard = await payments.card()
      await nextCard.attach('#commerce-card')
      setCard(nextCard)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not start checkout.')
    } finally {
      setBusy(false)
    }
  }

  const pay = async () => {
    setBusy(true)
    setError('')
    try {
      const result = await card.tokenize()
      if (result.status !== 'OK' || !result.token) throw new Error(result.errors?.[0]?.message || 'Card verification failed.')
      await client.subscribe(result.token, storeId, 'pro', attempt.current)
      setDone(true)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Subscription failed.')
    } finally {
      setBusy(false)
    }
  }

  if (done) return <Badge color="green">Subscribed</Badge>

  return (
    <div className="w-full max-w-sm">
      <div id="commerce-card" className={card ? 'mb-3 min-h-24' : 'hidden'} />
      <Button size="small" disabled={busy} onClick={card ? pay : load}>
        {busy ? 'Working…' : card ? 'Pay $20 and subscribe' : 'Add card'}
      </Button>
      {error && <Text size="xsmall" className="mt-2 text-ui-fg-error">{error}</Text>}
    </div>
  )
}
