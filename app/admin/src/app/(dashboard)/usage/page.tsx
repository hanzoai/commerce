'use client'

import { Component, useMemo, type ReactNode } from 'react'
import { Heading, Text, Container, Badge } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { StatCard } from '@/components/common/stat-card'
import { BarChart, Sparkline, Meter } from '@/components/usage/charts'
import { useList, useUsageAnalytics } from '@/lib/api/hooks'
import type { UsageRollup, Tier } from '@/lib/commerce-client'
import { formatMoney } from '@/lib/format'
import { bucketByDay, burnDown, groupBy, sumBy } from '@/lib/usage-series'

const WINDOW_DAYS = 14
const money = (cents: number) => formatMoney(Math.round(cents), 'usd')
const compactNumber = (n: number) =>
  new Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 }).format(n)

interface Order {
  id: string
  total?: number
  currency?: string
  paymentStatus?: string
  createdAt?: string
}

// A billing/usage read can throw inside a hook (org resolution, token refresh).
// Contain it so a merchant sees a clean message, never a white screen.
class UsageErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null }
  static getDerivedStateFromError(error: Error) { return { error } }
  render() {
    if (this.state.error) {
      return (
        <div>
          <PageHeader title="Usage & Analytics" description="Revenue, orders, AI spend, and credit usage" />
          <div className="p-8">
            <Text size="small" className="text-ui-fg-muted">
              Unable to load usage data. Please try refreshing the page.
            </Text>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

export default function UsagePage() {
  return (
    <UsageErrorBoundary>
      <UsageContent />
    </UsageErrorBoundary>
  )
}

function UsageContent() {
  const { data: orderList, isLoading: loadingOrders } = useList<Order>('order', { display: 200, sort: '-createdAt' })
  const { data: analytics, isLoading: loadingUsage, isError } = useUsageAnalytics()

  const orders = orderList?.models ?? []
  const usage = analytics?.usage.usage ?? []
  const rollup = analytics?.rollup ?? null
  const tier = analytics?.tier ?? null
  const loading = loadingOrders || loadingUsage

  // ── Series ────────────────────────────────────────────────────────────────
  const paidOrders = useMemo(
    () => orders.filter((o) => o.paymentStatus === 'captured' || o.paymentStatus === 'partially_captured'),
    [orders],
  )
  const revenueSeries = useMemo(
    () => bucketByDay(orders, (o) => o.createdAt, (o) => o.total ?? 0, WINDOW_DAYS),
    [orders],
  )
  const ordersSeries = useMemo(
    () => bucketByDay(orders, (o) => o.createdAt, () => 1, WINDOW_DAYS),
    [orders],
  )
  const spendSeries = useMemo(
    () => bucketByDay(usage, (u) => u.createdAt, (u) => Math.abs(u.amount ?? 0), WINDOW_DAYS),
    [usage],
  )

  // ── Totals ──────────────────────────────────────────────────────────────
  const revenueTotal = useMemo(() => sumBy(orders, (o) => o.total ?? 0), [orders])
  const aiSpendTotal = useMemo(() => sumBy(usage, (u) => Math.abs(u.amount ?? 0)), [usage])
  const tokenTotal = useMemo(() => sumBy(usage, (u) => Number(u.metadata?.totalTokens ?? 0)), [usage])
  const callCount = analytics?.usage.count ?? usage.length

  // Per-model AI spend breakdown from usage metadata.
  const byModel = useMemo(
    () => groupBy(usage, (u) => String(u.metadata?.model ?? 'unknown'), (u) => Math.abs(u.amount ?? 0)).slice(0, 6),
    [usage],
  )

  // Credit burn-down: start from the month's granted allowance, subtract daily
  // consumption. Falls back to remaining+consumed when granted is 0.
  const allowanceStart = rollup ? (rollup.included.grantedCents || rollup.included.remainingCents + rollup.consumedCents) : 0
  const burnSeries = useMemo(
    () => (allowanceStart > 0 ? burnDown(spendSeries, allowanceStart) : []),
    [spendSeries, allowanceStart],
  )

  const effectiveAvailable = tier?.balance?.effectiveAvailable ?? rollup?.balance.availableCents ?? 0
  const tierLabel = tier?.tier?.displayName || tier?.tier?.name || rollup?.plan || 'Pro'

  if (loading) return <LoadingState />

  const nothingYet = orders.length === 0 && usage.length === 0 && !rollup && !tier

  return (
    <div>
      <PageHeader
        title="Usage & Analytics"
        description="Revenue, orders, AI spend, and credit usage over the last 14 days"
        actions={<Badge color="grey">{tierLabel}</Badge>}
      />
      <div className="p-8">
        {isError && (
          <Container className="mb-6 p-4">
            <Text size="small" className="text-ui-fg-muted">
              Some usage figures are temporarily unavailable. Showing what we could load.
            </Text>
          </Container>
        )}

        {nothingYet ? (
          <Container className="p-10">
            <Heading level="h3">No activity yet</Heading>
            <Text size="small" className="mt-2 text-ui-fg-muted">
              Once you take orders and make AI or API calls, revenue, spend, and credit usage will appear here.
            </Text>
          </Container>
        ) : (
          <>
            <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard label="Revenue (14d)" value={money(revenueTotal)} />
              <StatCard label="Orders (14d)" value={orders.length} />
              <StatCard label="AI spend (14d)" value={money(aiSpendTotal)} />
              <StatCard label="Balance" value={money(effectiveAvailable)} />
            </div>

            <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
              <ChartCard title="Revenue over time" subtitle={`${money(revenueTotal)} across ${paidOrders.length} captured`}>
                <BarChart data={revenueSeries} format={money} colorClass="text-ui-tag-green-text" emptyLabel="No orders yet" />
              </ChartCard>
              <ChartCard title="Orders over time" subtitle={`${orders.length} orders`}>
                <BarChart data={ordersSeries} format={(v) => String(Math.round(v))} colorClass="text-ui-fg-base" emptyLabel="No orders yet" />
              </ChartCard>
            </div>

            <div className="mb-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
              <ChartCard title="AI spend over time" subtitle={`${callCount} calls · ${compactNumber(tokenTotal)} tokens`}>
                <BarChart data={spendSeries} format={money} colorClass="text-ui-tag-purple-text" emptyLabel="No AI usage yet" />
              </ChartCard>
              <ChartCard title="Credit burn-down" subtitle={allowanceStart > 0 ? `${money(allowanceStart)} monthly allowance` : 'No allowance granted'}>
                {burnSeries.length > 0 ? (
                  <div className="pt-6">
                    <Sparkline data={burnSeries} format={money} colorClass="text-ui-tag-orange-text" height={110} />
                  </div>
                ) : (
                  <BarChart data={[]} emptyLabel="No allowance to burn down" height={110} />
                )}
              </ChartCard>
            </div>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
              <TierCard tierLabel={tierLabel} rollup={rollup} tier={tier} />
              <ModelBreakdown byModel={byModel} total={aiSpendTotal} />
            </div>
          </>
        )}
      </div>
    </div>
  )
}

function ChartCard({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }) {
  return (
    <Container className="p-6">
      <div className="mb-4">
        <Heading level="h3">{title}</Heading>
        {subtitle && <Text size="small" className="mt-0.5 text-ui-fg-muted">{subtitle}</Text>}
      </div>
      {children}
    </Container>
  )
}

function TierCard({
  tierLabel,
  rollup,
  tier,
}: {
  tierLabel: string
  rollup: UsageRollup | null
  tier: Tier | null
}) {
  const included = rollup?.included
  const balance = tier?.balance
  const hasAllowance = !!included && (included.monthlyCents > 0 || included.grantedCents > 0)
  return (
    <Container className="p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <Heading level="h3">Plan & allowance</Heading>
        <Badge color="green">{tierLabel}</Badge>
      </div>

      {hasAllowance && included ? (
        <div className="mb-5">
          <Meter
            label="Included usage this month"
            used={included.consumedCents}
            total={included.grantedCents || included.monthlyCents}
            format={money}
            colorClass="bg-ui-fg-base"
          />
          {(rollup?.overageCents ?? 0) > 0 && (
            <Text size="xsmall" className="mt-2 text-ui-tag-orange-text">
              {money(rollup!.overageCents)} overage beyond the included allowance
            </Text>
          )}
        </div>
      ) : (
        <Text size="small" className="mb-5 text-ui-fg-muted">No included allowance on this plan.</Text>
      )}

      <div className="grid grid-cols-2 gap-4 border-t border-ui-border-base pt-4">
        <BalanceStat label="Prepaid" value={money(balance?.prepaidAvailable ?? 0)} />
        <BalanceStat label="Credits" value={money(balance?.creditsRemaining ?? 0)} />
        <BalanceStat label="Spendable" value={money(balance?.effectiveAvailable ?? rollup?.balance.availableCents ?? 0)} />
        <BalanceStat label="Consumed (mo)" value={money(rollup?.consumedCents ?? 0)} />
      </div>
    </Container>
  )
}

function BalanceStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <Text size="xsmall" className="text-ui-fg-muted">{label}</Text>
      <Text size="base" weight="plus" className="mt-0.5 text-ui-fg-base">{value}</Text>
    </div>
  )
}

function ModelBreakdown({ byModel, total }: { byModel: { key: string; value: number; count: number }[]; total: number }) {
  return (
    <Container className="p-6">
      <Heading level="h3" className="mb-4">AI spend by model</Heading>
      {byModel.length === 0 ? (
        <Text size="small" className="py-8 text-center text-ui-fg-muted">No metered AI usage yet</Text>
      ) : (
        <div className="space-y-3">
          {byModel.map((m) => {
            const pct = total > 0 ? (m.value / total) * 100 : 0
            return (
              <div key={m.key}>
                <div className="mb-1 flex items-baseline justify-between gap-2">
                  <Text as="span" size="small" className="truncate text-ui-fg-base">{m.key}</Text>
                  <Text as="span" size="small" weight="plus" className="shrink-0 text-ui-fg-base">
                    {money(m.value)} <span className="text-ui-fg-muted">· {m.count}</span>
                  </Text>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-ui-bg-component">
                  <div className="h-full rounded-full bg-ui-fg-base" style={{ width: `${Math.max(2, pct)}%` }} />
                </div>
              </div>
            )
          })}
        </div>
      )}
    </Container>
  )
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Usage & Analytics" description="Revenue, orders, AI spend, and credit usage over the last 14 days" />
      <div className="p-8">
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <Container key={i} className="p-6">
              <div className="h-4 w-24 animate-pulse rounded bg-ui-bg-component" />
              <div className="mt-3 h-8 w-20 animate-pulse rounded bg-ui-bg-component" />
            </Container>
          ))}
        </div>
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {[...Array(4)].map((_, i) => (
            <Container key={i} className="p-6">
              <div className="h-5 w-40 animate-pulse rounded bg-ui-bg-component" />
              <div className="mt-4 h-40 animate-pulse rounded bg-ui-bg-component" />
            </Container>
          ))}
        </div>
      </div>
    </div>
  )
}
