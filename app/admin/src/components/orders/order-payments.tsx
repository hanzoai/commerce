'use client'

import { StatusBadge, Table, Text } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { formatDate, formatMoney, titleCase } from '@/lib/format'
import { paymentStatusColor, type Payment } from './types'

// Presentational — the payments query is fired at the page level (in parallel with
// the order fetch) and passed down, so this panel never adds a fetch waterfall.
export function OrderPayments({
  payments,
  isLoading,
  currency,
}: {
  payments?: Payment[]
  isLoading?: boolean
  currency?: string
}) {
  return (
    <Section title="Payments">
      <div className="px-6 py-4">
      {isLoading ? (
        <Text size="small" className="text-ui-fg-subtle">
          Loading…
        </Text>
      ) : !payments || payments.length === 0 ? (
        <Text size="small" className="text-ui-fg-subtle">
          No payments recorded.
        </Text>
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Amount</Table.HeaderCell>
                <Table.HeaderCell>Status</Table.HeaderCell>
                <Table.HeaderCell className="text-right">Refunded</Table.HeaderCell>
                <Table.HeaderCell>Date</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {payments.map((payment) => (
                <Table.Row key={payment.id}>
                  <Table.Cell>{formatMoney(payment.amount, payment.currency ?? currency)}</Table.Cell>
                  <Table.Cell>
                    <StatusBadge color={paymentStatusColor(payment.status)}>
                      {titleCase(payment.status) || 'Unknown'}
                    </StatusBadge>
                  </Table.Cell>
                  <Table.Cell className="text-right">
                    {payment.amountRefunded ? formatMoney(payment.amountRefunded, payment.currency ?? currency) : '—'}
                  </Table.Cell>
                  <Table.Cell className="text-ui-fg-subtle">{formatDate(payment.createdAt)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </div>
      )}
      </div>
    </Section>
  )
}
