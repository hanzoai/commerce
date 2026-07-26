'use client'

// File a claim: pick an order, select the affected lines (quantity + per-line
// reason), choose how it settles (refund or replacement), then submit. The
// settled amount is computed server-side from the order's line prices — the form
// only expresses WHAT is claimed, never a money value. Item selection is local
// controlled state (a dynamic per-line picker); the submit posts one
// CreateClaimPayload to POST /v1/claim.

import { useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Input, Select, Switch, Text, Textarea, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { Field, Fieldset } from '@/components/common/field'
import { useCreate, useOrders, useOrder } from '@/lib/api/hooks'
import { formatMoney } from '@/lib/format'
import {
  lineId,
  lineLabel,
  REASONS,
  REASON_LABEL,
  RESOLUTIONS,
  RESOLUTION_LABEL,
  type Claim,
  type ClaimReason,
  type CreateClaimPayload,
  type OrderLine,
  type OrderLite,
  type Resolution,
} from '@/lib/claims/claim'

interface Selection {
  checked: boolean
  quantity: number
  reason: ClaimReason
}

export default function CreateClaimPage() {
  const router = useRouter()
  const create = useCreate<Claim>('claim')

  const [orderId, setOrderId] = useState('')
  const [resolution, setResolution] = useState<Resolution>('refund')
  const [summary, setSummary] = useState('')
  const [sel, setSel] = useState<Record<string, Selection>>({})

  const { data: orders, isLoading: ordersLoading } = useOrders({ display: 100 })
  const { data: order } = useOrder(orderId || undefined) as { data?: OrderLite }

  const lines = order?.items ?? []
  const currency = order?.currency || 'usd'

  const pick = (idKey: string): Selection => sel[idKey] ?? { checked: false, quantity: 1, reason: 'damaged' }
  const setPick = (idKey: string, patch: Partial<Selection>) =>
    setSel((s) => ({ ...s, [idKey]: { ...pick(idKey), ...patch } }))

  const chosen = useMemo(() => {
    return lines
      .map((line) => ({ line, id: lineId(line), s: pick(lineId(line)) }))
      .filter((x) => x.id && x.s.checked && x.s.quantity > 0)
  }, [lines, sel])

  const estimate = useMemo(
    () => chosen.reduce((sum, x) => sum + x.line.price * Math.min(x.s.quantity, x.line.quantity), 0),
    [chosen],
  )

  const onSelectOrder = (id: string) => {
    setOrderId(id)
    setSel({}) // a new order clears the line selection
  }

  const submit = async () => {
    if (!orderId) {
      toast.error('Select an order first')
      return
    }
    if (!chosen.length) {
      toast.error('Select at least one item')
      return
    }
    const payload: CreateClaimPayload = {
      orderId,
      resolution,
      reason: summary.trim() || undefined,
      currencyCode: currency,
      items: chosen.map((x) => ({
        itemId: x.id,
        quantity: Math.min(x.s.quantity, x.line.quantity),
        reason: x.s.reason,
      })),
    }
    try {
      const claim = await create.mutateAsync(payload)
      toast.success('Claim filed')
      router.push(`/claims/${claim.id}`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not file the claim')
    }
  }

  return (
    <div>
      <PageHeader title="File claim" description="Report damaged, wrong, or missing items on an order." />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        <Fieldset title="Order" description="The order the claim is filed against.">
          <Field label="Order">
            <Select value={orderId} onValueChange={onSelectOrder} disabled={ordersLoading}>
              <Select.Trigger>
                <Select.Value placeholder={ordersLoading ? 'Loading orders…' : 'Select an order'} />
              </Select.Trigger>
              <Select.Content>
                {(orders?.models ?? []).map((o: OrderLite) => (
                  <Select.Item key={o.id} value={o.id}>
                    {(o.number ? `#${o.number}` : o.id.slice(-8)) + (o.email ? ` · ${o.email}` : '')}
                  </Select.Item>
                ))}
              </Select.Content>
            </Select>
          </Field>
        </Fieldset>

        {orderId ? (
          <Fieldset title="Items" description="Select the affected lines, quantity, and reason.">
            {lines.length ? (
              <div className="flex flex-col gap-y-3">
                {lines.map((line) => (
                  <LineRow
                    key={lineId(line)}
                    line={line}
                    currency={currency}
                    selection={pick(lineId(line))}
                    onChange={(patch) => setPick(lineId(line), patch)}
                  />
                ))}
              </div>
            ) : (
              <Text size="small" className="text-ui-fg-muted">
                This order has no line items.
              </Text>
            )}
          </Fieldset>
        ) : null}

        <Fieldset title="Resolution" description="How an accepted claim settles.">
          <Field label="Resolution">
            <Select value={resolution} onValueChange={(v) => setResolution(v as Resolution)}>
              <Select.Trigger>
                <Select.Value />
              </Select.Trigger>
              <Select.Content>
                {RESOLUTIONS.map((r) => (
                  <Select.Item key={r} value={r}>
                    {RESOLUTION_LABEL[r]}
                  </Select.Item>
                ))}
              </Select.Content>
            </Select>
          </Field>
          <Field label="Note" optional hint="An optional summary of the claim.">
            <Textarea rows={3} placeholder="e.g. box arrived crushed" value={summary} onChange={(e) => setSummary(e.target.value)} />
          </Field>
        </Fieldset>

        <div className="flex items-center justify-between gap-2 border-t border-ui-border-base pt-4">
          <Text size="small" className="text-ui-fg-subtle">
            {chosen.length ? `Estimated ${formatMoney(estimate, currency)} across ${chosen.length} line(s)` : 'No items selected'}
          </Text>
          <div className="flex items-center gap-2">
            <Button type="button" variant="secondary" size="small" onClick={() => router.push('/claims')} disabled={create.isPending}>
              Cancel
            </Button>
            <Button type="button" size="small" onClick={submit} isLoading={create.isPending} disabled={!chosen.length}>
              File claim
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

function LineRow({
  line,
  currency,
  selection,
  onChange,
}: {
  line: OrderLine
  currency: string
  selection: Selection
  onChange: (patch: Partial<Selection>) => void
}) {
  return (
    <div className="flex flex-col gap-y-3 rounded-lg border border-ui-border-base bg-ui-bg-base p-3 sm:flex-row sm:items-center sm:gap-x-4">
      <div className="flex items-start gap-x-3">
        <Switch checked={selection.checked} onCheckedChange={(checked) => onChange({ checked })} />
        <div className="min-w-0">
          <Text size="small" weight="plus" className="text-ui-fg-base">
            {lineLabel(line)}
          </Text>
          <Text size="small" leading="compact" className="text-ui-fg-subtle">
            {line.quantity} × {formatMoney(line.price, currency)}
          </Text>
        </div>
      </div>
      {selection.checked ? (
        <div className="grid flex-1 grid-cols-2 gap-3 sm:max-w-xs">
          <label className="flex flex-col gap-y-1">
            <Text size="xsmall" className="text-ui-fg-muted">
              Quantity
            </Text>
            <Input
              type="number"
              min={1}
              max={line.quantity}
              value={String(selection.quantity)}
              onChange={(e) => onChange({ quantity: Math.max(1, Math.min(line.quantity, Number(e.target.value) || 1)) })}
            />
          </label>
          <label className="flex flex-col gap-y-1">
            <Text size="xsmall" className="text-ui-fg-muted">
              Reason
            </Text>
            <Select value={selection.reason} onValueChange={(reason) => onChange({ reason: reason as ClaimReason })}>
              <Select.Trigger>
                <Select.Value />
              </Select.Trigger>
              <Select.Content>
                {REASONS.map((r) => (
                  <Select.Item key={r} value={r}>
                    {REASON_LABEL[r]}
                  </Select.Item>
                ))}
              </Select.Content>
            </Select>
          </label>
        </div>
      ) : null}
    </div>
  )
}
