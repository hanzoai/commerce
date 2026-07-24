'use client'

// Inline edit form for an order — status, contact, gift note, and both addresses.
// Loaded on demand (dynamic import) from the detail page so viewing an order never
// ships the react-hook-form + zod chunk. PATCHes /v1/order/:id via useUpdate.
import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, toast, usePrompt } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { useDelete, useUpdate } from '@/lib/api/hooks'
import { cleanEmpty, errorMessage } from '@/lib/forms/schema'
import { orderEditDefaults, orderEditSchema, type OrderEditValues } from './order-form'
import { AddressFields, OrderDetailsFields } from './order-fields'
import type { Order } from './types'

export function OrderEditForm({ order, onDone }: { order: Order; onDone: () => void }) {
  const router = useRouter()
  const prompt = usePrompt()
  const update = useUpdate<Order>('order')
  const remove = useDelete('order')

  const form = useForm<OrderEditValues>({
    resolver: zodResolver(orderEditSchema),
    defaultValues: orderEditDefaults(order),
  })

  const submit = form.handleSubmit(async (values) => {
    try {
      await update.mutateAsync({ id: order.id, data: cleanEmpty(values) as Partial<Order> })
      toast.success('Order updated')
      onDone()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not update order'))
    }
  })

  const handleDelete = async () => {
    const confirmed = await prompt({
      title: 'Delete order',
      description: `Delete order #${order.number ?? order.id}? This permanently removes it and cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await remove.mutateAsync(order.id)
      toast.success('Order deleted')
      router.push('/orders')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not delete order'))
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-y-4">
      <Section title="Edit order">
        <div className="px-6 py-4">
          <OrderDetailsFields control={form.control} />
        </div>
      </Section>
      <Section title="Shipping address">
        <div className="px-6 py-4">
          <AddressFields control={form.control} prefix="shippingAddress" />
        </div>
      </Section>
      <Section title="Billing address">
        <div className="px-6 py-4">
          <AddressFields control={form.control} prefix="billingAddress" />
        </div>
      </Section>
      <div className="flex items-center justify-between gap-2">
        <Button type="button" size="small" variant="danger" onClick={handleDelete} isLoading={remove.isPending}>
          Delete
        </Button>
        <div className="flex items-center gap-2">
          <Button type="button" size="small" variant="secondary" onClick={onDone} disabled={update.isPending}>
            Cancel
          </Button>
          <Button type="submit" size="small" isLoading={update.isPending}>
            Save changes
          </Button>
        </div>
      </div>
    </form>
  )
}

export default OrderEditForm
