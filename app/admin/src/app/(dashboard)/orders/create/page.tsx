'use client'

import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Container, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { useCreate } from '@/lib/api/hooks'
import { cleanEmpty, errorMessage } from '@/lib/forms/schema'
import { orderCreateDefaults, orderCreateSchema, type OrderCreateValues } from '@/components/orders/order-form'
import { OrderDetailsFields } from '@/components/orders/order-fields'
import type { Order } from '@/components/orders/types'

export default function CreateOrderPage() {
  const router = useRouter()
  const { mutateAsync, isPending } = useCreate<Order>('order')

  const form = useForm<OrderCreateValues>({
    resolver: zodResolver(orderCreateSchema),
    defaultValues: orderCreateDefaults(),
  })

  const submit = form.handleSubmit(async (values) => {
    try {
      await mutateAsync(cleanEmpty(values) as Partial<Order>)
      toast.success('Order created')
      router.push('/orders')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not create order'))
    }
  })

  return (
    <div>
      <ToasterMount />
      <PageHeader title="Create order" description="Manually record an order" />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          <form onSubmit={submit} className="flex flex-col gap-y-6">
            <OrderDetailsFields control={form.control} currency />
            <div className="flex items-center justify-end gap-2">
              <Button type="button" size="small" variant="secondary" onClick={() => router.push('/orders')} disabled={isPending}>
                Cancel
              </Button>
              <Button type="submit" size="small" isLoading={isPending}>
                Create order
              </Button>
            </div>
          </form>
        </Container>
      </div>
    </div>
  )
}
