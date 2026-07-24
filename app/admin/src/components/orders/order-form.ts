import { z } from 'zod'
import { emailField, optionalText } from '@/lib/forms/schema'
import { titleCase } from '@/lib/format'
import { ORDER_STATUSES, type Order, type OrderStatus } from './types'

// Order form schemas + defaults, shared by the create page and the inline edit
// form. Composed from the same field helpers every resource form uses, so
// validation rules live in one place.

const addressObject = z.object({
  name: optionalText,
  line1: optionalText,
  line2: optionalText,
  city: optionalText,
  state: optionalText,
  postalCode: optionalText,
  country: optionalText,
})

export const CURRENCY_OPTIONS: { value: string; label: string }[] = [
  { value: 'usd', label: 'USD' },
  { value: 'eur', label: 'EUR' },
  { value: 'gbp', label: 'GBP' },
  { value: 'cad', label: 'CAD' },
  { value: 'aud', label: 'AUD' },
]

export const STATUS_OPTIONS = ORDER_STATUSES.map((status) => ({ value: status, label: titleCase(status) }))

export const orderCreateSchema = z.object({
  email: emailField,
  currency: z.string().trim().min(1, 'Required'),
  status: z.enum(ORDER_STATUSES),
  company: optionalText,
  giftMessage: optionalText,
})
export type OrderCreateValues = z.infer<typeof orderCreateSchema>

export const orderCreateDefaults = (): OrderCreateValues => ({
  email: '',
  currency: 'usd',
  status: 'open',
  company: '',
  giftMessage: '',
})

export const orderEditSchema = z.object({
  // Edit forms may clear the email, so an empty string is allowed here.
  email: z.string().trim().email('Enter a valid email').or(z.literal('')).optional(),
  status: z.enum(ORDER_STATUSES),
  company: optionalText,
  giftMessage: optionalText,
  shippingAddress: addressObject,
  billingAddress: addressObject,
})
export type OrderEditValues = z.infer<typeof orderEditSchema>

const fillAddress = (address?: Order['shippingAddress']): z.infer<typeof addressObject> => ({
  name: address?.name ?? '',
  line1: address?.line1 ?? '',
  line2: address?.line2 ?? '',
  city: address?.city ?? '',
  state: address?.state ?? '',
  postalCode: address?.postalCode ?? '',
  country: address?.country ?? '',
})

const coerceStatus = (status?: string): OrderStatus =>
  (ORDER_STATUSES as readonly string[]).includes(status ?? '') ? (status as OrderStatus) : 'open'

export const orderEditDefaults = (order: Order): OrderEditValues => ({
  email: order.email ?? '',
  status: coerceStatus(order.status),
  company: order.company ?? '',
  giftMessage: order.giftMessage ?? '',
  shippingAddress: fillAddress(order.shippingAddress),
  billingAddress: fillAddress(order.billingAddress),
})
