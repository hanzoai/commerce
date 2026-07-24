'use client'

import { useState } from 'react'
import dynamic from 'next/dynamic'
import { useParams, useRouter } from 'next/navigation'
import { Button, StatusBadge, Text, toast, usePrompt } from '@hanzo/commerce-ui'
import { DetailShell } from '@/components/common/detail-view/detail-shell'
import { ToasterMount } from '@/components/common/toaster-mount'
import { useDelete, useGet, useResourceActionData } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import { titleCase } from '@/lib/format'
import { orderStatusColor, paymentStatusColor, type Order, type Payment } from '@/components/orders/types'
import { OrderSummary } from '@/components/orders/order-summary'
import { OrderCustomer } from '@/components/orders/order-customer'
import { OrderPayments } from '@/components/orders/order-payments'

// Editing (react-hook-form + zod) and the action dialogs are rarely-used panels —
// defer their chunks so viewing an order paints fast.
const OrderEditForm = dynamic(() => import('@/components/orders/order-edit-form').then((m) => m.OrderEditForm), {
  ssr: false,
  loading: () => (
    <div className="px-6 py-6">
      <Text size="small" className="text-ui-fg-muted">
        Loading…
      </Text>
    </div>
  ),
})
const OrderActions = dynamic(() => import('@/components/orders/order-actions').then((m) => m.OrderActions), {
  ssr: false,
  loading: () => null,
})

export default function OrderDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()
  const prompt = usePrompt()
  const [editing, setEditing] = useState(false)

  // Fetch the order and its payments in parallel — no waterfall.
  const { data: order, isLoading, isError } = useGet<Order>('order', id)
  const payments = useResourceActionData<Payment[]>('order', id, 'payments')
  const { mutateAsync: remove } = useDelete('order')

  const handleDelete = async () => {
    if (!order) return
    const confirmed = await prompt({
      title: 'Delete order',
      description: `Delete order #${order.number ?? order.id}? This permanently removes it and cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await remove(order.id)
      toast.success('Order deleted')
      router.push('/orders')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not delete order'))
    }
  }

  const title = order ? `Order #${order.number ?? order.id.slice(-6)}` : 'Order'

  const actions = order ? (
    <>
      <OrderActions order={order} />
      <Button size="small" variant="secondary" onClick={() => setEditing((v) => !v)}>
        {editing ? 'Close' : 'Edit'}
      </Button>
      <Button size="small" variant="danger" onClick={handleDelete}>
        Delete
      </Button>
    </>
  ) : null

  return (
    <>
      <ToasterMount />
      <DetailShell
        title={title}
        subtitle={order?.email || 'Order'}
        backHref="/orders"
        backLabel="Orders"
        actions={actions}
        isLoading={isLoading}
        notFound={!isLoading && (isError || !order)}
        notFoundLabel="This order could not be found."
      >
        {order && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <StatusBadge color={orderStatusColor(order.status)}>{titleCase(order.status) || 'Open'}</StatusBadge>
              {order.paymentStatus && (
                <StatusBadge color={paymentStatusColor(order.paymentStatus)}>{titleCase(order.paymentStatus)}</StatusBadge>
              )}
            </div>
            {editing && <OrderEditForm order={order} onDone={() => setEditing(false)} />}
            <OrderSummary order={order} />
            <OrderCustomer order={order} />
            <OrderPayments payments={payments.data} isLoading={payments.isLoading} currency={order.currency} />
          </>
        )}
      </DetailShell>
    </>
  )
}
