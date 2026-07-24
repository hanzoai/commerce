'use client'

// Order money/logistics actions: refund, create-fulfillment → ship, and a status
// check. Each hits its real endpoint through the generic resource-action hooks
// (POST /v1/order/:id/refund, POST /v1/fulfillment, POST /v1/fulfillment/:id/ship,
// GET /v1/order/:id/status). Loaded on demand from the detail page — the dialogs
// and their state never ship until an admin opens the panel.
import { useState, type ReactNode } from 'react'
import { Button, Heading, Input, Text, toast } from '@hanzo/commerce-ui'
import { Field } from '@/components/common/field'
import { useCreate, useResourceAction, useResourceActionData } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import { amountToCents, centsToAmount, formatMoney, titleCase } from '@/lib/format'
import { lineItemName, lineItemSku, type Order, type OrderStatusResponse } from './types'

interface FulfillmentItem {
  title: string
  sku: string
  quantity: number
  lineItemId?: string
}
interface Fulfillment {
  id: string
  orderId?: string
  items?: FulfillmentItem[]
}
interface ShipBody {
  labels: { trackingNumber: string; trackingUrl: string }[]
}

function ActionModal({
  title,
  children,
  submitLabel,
  busy,
  onClose,
  onSubmit,
}: {
  title: string
  children: ReactNode
  submitLabel: string
  busy?: boolean
  onClose: () => void
  onSubmit: () => void
}) {
  return (
    <div
      className="fixed inset-0 z-[80] flex items-center justify-center bg-ui-bg-overlay p-4"
      role="presentation"
      onMouseDown={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        className="w-full max-w-md rounded-xl border border-ui-border-base bg-ui-bg-subtle p-6 shadow-elevation-modal"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <Heading level="h2">{title}</Heading>
        <div className="mt-4 flex flex-col gap-3">{children}</div>
        <div className="mt-6 flex justify-end gap-2">
          <Button size="small" variant="secondary" onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button size="small" onClick={onSubmit} isLoading={busy}>
            {submitLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}

export function OrderActions({ order }: { order: Order }) {
  const [open, setOpen] = useState<null | 'refund' | 'fulfill'>(null)
  const [amount, setAmount] = useState('')
  const [fulfillmentId, setFulfillmentId] = useState<string | null>(null)
  const [tracking, setTracking] = useState({ number: '', url: '' })

  const refund = useResourceAction<Order, { amount: number }>('order', order.id, 'refund')
  const createFulfillment = useCreate<Fulfillment>('fulfillment')
  const ship = useResourceAction<Fulfillment, ShipBody>('fulfillment', fulfillmentId ?? undefined, 'ship')
  const status = useResourceActionData<OrderStatusResponse>('order', order.id, 'status', { enabled: false })

  const remaining = (order.total ?? 0) - (order.refunded ?? 0)

  const openRefund = () => {
    setAmount(centsToAmount(remaining))
    setOpen('refund')
  }
  const openFulfill = () => {
    setFulfillmentId(null)
    setTracking({ number: '', url: '' })
    setOpen('fulfill')
  }

  const doRefund = async () => {
    const cents = amountToCents(amount)
    if (cents <= 0) {
      toast.error('Enter a refund amount')
      return
    }
    try {
      await refund.mutateAsync({ amount: cents })
      toast.success(`Refunded ${formatMoney(cents, order.currency)}`)
      setOpen(null)
    } catch (e) {
      toast.error(errorMessage(e, 'Refund failed'))
    }
  }

  const doCreateFulfillment = async () => {
    const items: FulfillmentItem[] = (order.items ?? []).map((li) => ({
      title: lineItemName(li),
      sku: lineItemSku(li),
      quantity: li.quantity,
      lineItemId: li.variantId || li.productId,
    }))
    try {
      const created = await createFulfillment.mutateAsync({ orderId: order.id, items })
      setFulfillmentId(created.id)
      toast.success('Fulfillment created')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not create fulfillment'))
    }
  }

  const doShip = async () => {
    try {
      await ship.mutateAsync({ labels: [{ trackingNumber: tracking.number, trackingUrl: tracking.url }] })
      toast.success('Marked as shipped')
      setOpen(null)
    } catch (e) {
      toast.error(errorMessage(e, 'Could not mark as shipped'))
    }
  }

  const checkStatus = async () => {
    try {
      const { data } = await status.refetch()
      if (data) {
        toast.success(`Status: ${titleCase(data.status) || 'Unknown'} · paid ${formatMoney(data.paid, data.currency ?? order.currency)}`)
      }
    } catch (e) {
      toast.error(errorMessage(e, 'Could not load status'))
    }
  }

  return (
    <>
      <Button size="small" variant="secondary" onClick={checkStatus} isLoading={status.isFetching}>
        Status
      </Button>
      <Button size="small" variant="secondary" onClick={openFulfill}>
        Fulfill
      </Button>
      <Button size="small" variant="secondary" onClick={openRefund}>
        Refund
      </Button>

      {open === 'refund' && (
        <ActionModal
          title="Issue refund"
          submitLabel="Refund"
          busy={refund.isPending}
          onClose={() => setOpen(null)}
          onSubmit={doRefund}
        >
          <Field label="Amount" hint={`Remaining ${formatMoney(remaining, order.currency)}`}>
            <Input value={amount} onChange={(event) => setAmount(event.target.value)} inputMode="decimal" placeholder="0.00" />
          </Field>
        </ActionModal>
      )}

      {open === 'fulfill' && (
        <ActionModal
          title="Fulfillment"
          submitLabel={fulfillmentId ? 'Mark shipped' : 'Create fulfillment'}
          busy={createFulfillment.isPending || ship.isPending}
          onClose={() => setOpen(null)}
          onSubmit={fulfillmentId ? doShip : doCreateFulfillment}
        >
          {!fulfillmentId ? (
            <Text size="small" className="text-ui-fg-subtle">
              Create a fulfillment for all {order.items?.length ?? 0} item(s) in this order.
            </Text>
          ) : (
            <>
              <Text size="small" className="text-ui-fg-subtle">
                Fulfillment created. Add optional tracking, then mark it shipped.
              </Text>
              <Field label="Tracking number" optional>
                <Input
                  value={tracking.number}
                  onChange={(event) => setTracking((prev) => ({ ...prev, number: event.target.value }))}
                />
              </Field>
              <Field label="Tracking URL" optional>
                <Input
                  value={tracking.url}
                  onChange={(event) => setTracking((prev) => ({ ...prev, url: event.target.value }))}
                />
              </Field>
            </>
          )}
        </ActionModal>
      )}
    </>
  )
}

export default OrderActions
