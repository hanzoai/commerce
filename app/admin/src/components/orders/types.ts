// Order shapes mirror the live Go `/v1/order` model (models/order/order.go).
// The data-provider hooks are typed `any`, so these give the order pages a real
// contract without over-committing to fields the UI never reads.

export interface Address {
  name?: string
  line1?: string
  line2?: string
  city?: string
  state?: string
  postalCode?: string
  country?: string
}

export interface LineItem {
  productId?: string
  productName?: string
  productSlug?: string
  productSKU?: string
  variantId?: string
  variantName?: string
  variantSKU?: string
  sku?: string
  quantity: number
  price?: number
  free?: boolean
}

export interface OrderFulfillment {
  type?: string
  status?: string
  carrier?: string
  service?: string
}

export interface Order {
  id: string
  number?: number
  email?: string
  company?: string
  userId?: string
  status?: string
  paymentStatus?: string
  currency?: string
  lineTotal?: number
  subtotal?: number
  discount?: number
  shipping?: number
  tax?: number
  total?: number
  paid?: number
  refunded?: number
  balance?: number
  billingAddress?: Address
  shippingAddress?: Address
  items?: LineItem[]
  payments?: string[]
  fulfillment?: OrderFulfillment
  giftMessage?: string
  createdAt?: string
  metadata?: Record<string, unknown>
}

export interface Payment {
  id: string
  amount?: number
  amountRefunded?: number
  currency?: string
  status?: string
  description?: string
  captured?: boolean
  createdAt?: string
}

export interface OrderStatusResponse {
  id: string
  total?: number
  paid?: number
  currency?: string
  status?: string
  paymentStatus?: string
}

// order.Status enum (models/order/order.go)
export const ORDER_STATUSES = ['open', 'completed', 'cancelled', 'locked', 'on-hold'] as const
export type OrderStatus = (typeof ORDER_STATUSES)[number]

type BadgeColor = 'green' | 'orange' | 'red' | 'grey' | 'blue' | 'purple'

export function orderStatusColor(status?: string): BadgeColor {
  switch (status) {
    case 'completed':
      return 'green'
    case 'open':
      return 'blue'
    case 'on-hold':
    case 'locked':
      return 'orange'
    case 'cancelled':
      return 'red'
    default:
      return 'grey'
  }
}

// payment.Status enum (models/payment/payment.go)
export function paymentStatusColor(status?: string): BadgeColor {
  switch (status) {
    case 'paid':
    case 'credit':
      return 'green'
    case 'disputed':
      return 'orange'
    case 'failed':
    case 'fraudulent':
    case 'cancelled':
    case 'refunded':
      return 'red'
    default:
      return 'grey'
  }
}

export const lineItemName = (li: LineItem) =>
  li.variantName || li.productName || li.productSlug || li.productSKU || li.sku || 'Item'

export const lineItemSku = (li: LineItem) => li.variantSKU || li.productSKU || li.sku || ''
