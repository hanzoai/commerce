'use client'

import { use } from 'react'
import Link from 'next/link'
import { Badge, Button, Heading, Text, Container } from '@hanzo/commerce-ui'
import { useOrder } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'

export function OrderDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { data: order, isLoading } = useOrder(id)

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Loading..." />
        <div className="p-8">
          <div className="space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 animate-pulse rounded-lg bg-ui-bg-component" />
            ))}
          </div>
        </div>
      </div>
    )
  }

  if (!order) {
    return (
      <div>
        <PageHeader title="Order Not Found" />
        <div className="p-8 text-center">
          <Text size="small" className="text-ui-fg-muted">This order doesn&apos;t exist or you don&apos;t have access.</Text>
          <Button variant="secondary" className="mt-4" asChild>
            <Link href="/orders">Back to Orders</Link>
          </Button>
        </div>
      </div>
    )
  }

  const currency = order.currency || 'USD'
  const fmt = (v: number) =>
    new Intl.NumberFormat('en-US', { style: 'currency', currency }).format(v / 100)

  return (
    <div>
      <PageHeader
        title={`Order #${order.number || order.id?.slice(-6)}`}
        description={order.email || undefined}
        actions={
          <Button variant="secondary" asChild>
            <Link href="/orders">Back to Orders</Link>
          </Button>
        }
      />
      <div className="p-8">
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <Container className="p-6">
            <Heading level="h3" className="mb-4">Status</Heading>
            <div className="space-y-3">
              <div>
                <Text as="span" size="xsmall" className="text-ui-fg-muted">Order Status</Text>
                <div className="mt-1">
                  <Badge color={order.status === 'completed' ? 'green' : 'orange'}>
                    {order.status || 'pending'}
                  </Badge>
                </div>
              </div>
              <div>
                <Text as="span" size="xsmall" className="text-ui-fg-muted">Payment</Text>
                <div className="mt-1">
                  <Badge color={order.paymentStatus === 'paid' ? 'green' : 'orange'}>
                    {order.paymentStatus || 'pending'}
                  </Badge>
                </div>
              </div>
              <div>
                <Text as="span" size="xsmall" className="text-ui-fg-muted">Fulfillment</Text>
                <div className="mt-1">
                  <Badge color={order.fulfillmentStatus === 'fulfilled' ? 'green' : 'orange'}>
                    {order.fulfillmentStatus || 'unfulfilled'}
                  </Badge>
                </div>
              </div>
            </div>
          </Container>

          <Container className="p-6">
            <Heading level="h3" className="mb-4">Summary</Heading>
            <dl className="space-y-3">
              <Field label="Subtotal" value={order.subtotal != null ? fmt(order.subtotal) : '-'} />
              <Field label="Shipping" value={order.shippingTotal != null ? fmt(order.shippingTotal) : '-'} />
              <Field label="Tax" value={order.taxTotal != null ? fmt(order.taxTotal) : '-'} />
              <Field label="Total" value={order.total != null ? fmt(order.total) : '-'} bold />
            </dl>
          </Container>

          <Container className="p-6">
            <Heading level="h3" className="mb-4">Customer</Heading>
            <dl className="space-y-3">
              <Field label="Email" value={order.email} />
              <Field label="Created" value={order.createdAt ? new Date(order.createdAt).toLocaleString() : '-'} />
            </dl>
          </Container>
        </div>

        {order.items?.length > 0 && (
          <Container className="mt-6 p-6">
            <Heading level="h3" className="mb-4">Line Items</Heading>
            <table className="w-full">
              <thead>
                <tr className="border-b border-ui-border-base text-left">
                  <th className="pb-2"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Product</Text></th>
                  <th className="pb-2"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Qty</Text></th>
                  <th className="pb-2 text-right"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Unit Price</Text></th>
                  <th className="pb-2 text-right"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Total</Text></th>
                </tr>
              </thead>
              <tbody>
                {order.items.map((item: any, i: number) => (
                  <tr key={i} className="border-b border-ui-border-base last:border-0">
                    <td className="py-3"><Text as="span" size="small">{item.title || item.name || '-'}</Text></td>
                    <td className="py-3"><Text as="span" size="small" className="text-ui-fg-muted">{item.quantity ?? 1}</Text></td>
                    <td className="py-3 text-right"><Text as="span" size="small" className="text-ui-fg-muted">{item.unitPrice != null ? fmt(item.unitPrice) : '-'}</Text></td>
                    <td className="py-3 text-right"><Text as="span" size="small">{item.total != null ? fmt(item.total) : '-'}</Text></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Container>
        )}
      </div>
    </div>
  )
}

function Field({ label, value, bold }: { label: string; value?: string | null; bold?: boolean }) {
  return (
    <div>
      <Text as="span" size="xsmall" className="text-ui-fg-muted">{label}</Text>
      <Text size="small" weight={bold ? 'plus' : 'regular'} className="mt-0.5 text-ui-fg-base">{value || '-'}</Text>
    </div>
  )
}
