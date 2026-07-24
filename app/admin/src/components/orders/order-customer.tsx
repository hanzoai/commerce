'use client'

import { memo } from 'react'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import type { Address, Order } from './types'

// SectionRow renders a string value with `whitespace-pre-line`, so a newline-joined
// address shows one line per part.
function formatAddress(address?: Address): string {
  if (!address) return ''
  const cityLine = [address.city, address.state, address.postalCode].filter(Boolean).join(', ')
  return [address.name, address.line1, address.line2, cityLine, address.country].filter(Boolean).join('\n')
}

export const OrderCustomer = memo(function OrderCustomer({ order }: { order: Order }) {
  const shipping = formatAddress(order.shippingAddress)
  const billing = formatAddress(order.billingAddress)

  return (
    <Section title="Customer">
      <SectionRow title="Email" value={order.email || '-'} />
      {order.company ? <SectionRow title="Company" value={order.company} /> : null}
      <SectionRow title="Shipping address" value={shipping || '-'} />
      <SectionRow title="Billing address" value={billing || '-'} />
    </Section>
  )
})
