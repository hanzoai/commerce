'use client'

import { useEffect, useState } from 'react'
import { Commerce } from '@hanzo/commerce-client'
import { Heading, Text, Container } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { PageHeader } from '@/components/common/page-header'
import { StatCard } from '@/components/common/stat-card'

export default function BillingPage() {
  const { accessToken: token, isAuthenticated } = useIam()
  const { currentOrgId } = useOrganizations()
  const [balance, setBalance] = useState<any>(null)
  const [creditBalance, setCreditBalance] = useState<any>(null)
  const [invoices, setInvoices] = useState<any[]>([])
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
    ]).then(([balRes, creditRes, invRes]) => {
      if (balRes.status === 'fulfilled') setBalance(balRes.value)
      if (creditRes.status === 'fulfilled') setCreditBalance(creditRes.value)
      if (invRes.status === 'fulfilled') setInvoices(invRes.value)
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
