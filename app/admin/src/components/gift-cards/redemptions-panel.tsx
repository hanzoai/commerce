'use client'

// The append-only redemption ledger on the gift-card detail page. Dynamically
// imported (off the first paint) and self-fetching (GET
// /v1/gift-card/:id/redemptions) so it loads in parallel with the balance. Each
// active debit can be VOIDED — a compensating reversal line, confirmed first —
// which re-reads the balance + ledger via the action hook's invalidation.

import { Badge, Table, Text, toast } from '@hanzo/commerce-ui'
import { Fieldset } from '@/components/common/field'
import { ConfirmButton } from '@/components/common/confirm-button'
import { useResourceAction, useResourceActionData } from '@/lib/api/hooks'
import { formatDate, formatMoney } from '@/lib/format'
import type { GiftCardRedemption, VoidResult } from '@/lib/gift-cards/gift-card'

export function RedemptionsPanel({ id, currency }: { id: string; currency: string }) {
  const { data, isLoading } = useResourceActionData<GiftCardRedemption[]>('gift-card', id, 'redemptions')
  const voidAction = useResourceAction<VoidResult, { redemptionId: string }>('gift-card', id, 'void')
  const rows = data ?? []

  const onVoid = async (redemptionId: string) => {
    try {
      const res = await voidAction.mutateAsync({ redemptionId })
      toast.success(`Voided — ${formatMoney(res.balanceCents, currency)} available`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Void failed')
    }
  }

  return (
    <Fieldset title="Redemptions" description="Append-only debit ledger. Balance = initial − Σ redemptions.">
      {isLoading ? (
        <Text size="small" className="text-ui-fg-subtle">
          Loading redemptions…
        </Text>
      ) : rows.length === 0 ? (
        <Text size="small" className="text-ui-fg-subtle">
          No redemptions yet.
        </Text>
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell>Amount</Table.HeaderCell>
                <Table.HeaderCell>Type</Table.HeaderCell>
                <Table.HeaderCell>Order</Table.HeaderCell>
                <Table.HeaderCell>When</Table.HeaderCell>
                <Table.HeaderCell />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {rows.map((r) => (
                <Table.Row key={r.id}>
                  <Table.Cell>{formatMoney(r.amountCents, r.currency || currency)}</Table.Cell>
                  <Table.Cell>
                    {r.isReversal ? <Badge color="orange">Reversal</Badge> : <Badge color="blue">Debit</Badge>}
                  </Table.Cell>
                  <Table.Cell>{r.orderId || '—'}</Table.Cell>
                  <Table.Cell>{formatDate(r.createdAt)}</Table.Cell>
                  <Table.Cell>
                    {!r.isReversal && r.amountCents > 0 && (
                      <ConfirmButton
                        variant="secondary"
                        onConfirm={() => onVoid(r.id)}
                        title="Void redemption"
                        description="This reverses the debit with a compensating entry and restores the balance."
                        confirmText="Void"
                      >
                        Void
                      </ConfirmButton>
                    )}
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </div>
      )}
    </Fieldset>
  )
}
