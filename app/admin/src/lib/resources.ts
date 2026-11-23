/**
 * The admin's resource catalog — DATA, and only data.
 *
 * Every merchant and payments surface is the same thing: a `/v1/commerce/<kind>`
 * or `/v1/billing/<kind>` list rendered by the ONE shared `CommerceResource` from
 * `@hanzo/ui/product`. Adding a surface means adding a row here — the sidebar,
 * the ⌘K palette, and static route generation all read this list.
 *
 * Deliberately free of any component import: `generateStaticParams` runs this
 * module in Node at build time.
 */

/** Any row the API returns — resources are described by columns, not by types. */
export type Row = Record<string, unknown>

/** How a cell reads: emphasised first column, muted text, date, money (minor units), count, status tag, or flag. */
export type ColumnSpec = {
  key: string
  header: string
  as: 'name' | 'text' | 'date' | 'money' | 'num' | 'status' | 'flag'
  /** `flag` only — the status shown when the value is truthy / falsy. */
  on?: string
  off?: string
}

export type Resource = {
  /** URL slug AND the sidebar key. */
  slug: string
  /** Backend noun (`/v1/commerce/<kind>`) or explicit `/v1/...` path. */
  kind: string
  label: string
  subtitle: string
  /** Honest empty copy for an org that has none of these yet. */
  empty: string
  columns: ColumnSpec[]
}

export const RESOURCES: Resource[] = [
  {
    slug: 'payments',
    kind: '/v1/billing/transactions',
    label: 'Payments',
    subtitle: 'Incoming payments, top-ups, charges, and wallet transactions for this organization',
    empty: 'No payments or transactions recorded for this organization yet.',
    columns: [
      { key: 'id', header: 'Transaction', as: 'name' },
      { key: 'type', header: 'Type', as: 'status' },
      { key: 'description', header: 'Description', as: 'text' },
      { key: 'amount', header: 'Amount', as: 'money' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'createdAt', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'ledger',
    kind: '/v1/billing/ledger',
    label: 'Ledger',
    subtitle: 'Deposits, holds, and metered usage postings across the last 90 days',
    empty: 'No ledger entries in the last 90 days.',
    columns: [
      { key: 'id', header: 'Entry', as: 'name' },
      { key: 'account', header: 'Account', as: 'text' },
      { key: 'description', header: 'Description', as: 'text' },
      { key: 'cents', header: 'Amount', as: 'money' },
      { key: 'date', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'orders',
    kind: 'order',
    label: 'Orders',
    subtitle: 'Customer storefront orders and checkout revenue for this organization',
    empty: 'No orders yet.',
    columns: [
      { key: 'number', header: 'Order', as: 'name' },
      { key: 'email', header: 'Customer', as: 'text' },
      { key: 'total', header: 'Total', as: 'money' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'paymentStatus', header: 'Payment', as: 'status' },
      { key: 'createdAt', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'subscriptions',
    kind: '/v1/billing/subscriptions',
    label: 'Subscriptions',
    subtitle: 'Active and historical recurring plans and MRR for this organization',
    empty: 'No subscriptions yet.',
    columns: [
      { key: 'planName', header: 'Plan', as: 'name' },
      { key: 'userId', header: 'Account', as: 'text' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'mrrCents', header: 'MRR', as: 'money' },
      { key: 'providerType', header: 'Processor', as: 'text' },
      { key: 'currentPeriodEnd', header: 'Renews', as: 'date' },
    ],
  },
  {
    slug: 'invoices',
    kind: '/v1/billing/invoices',
    label: 'Invoices',
    subtitle: 'Issued, paid, and open invoices for this organization',
    empty: 'No invoices yet.',
    columns: [
      { key: 'number', header: 'Invoice', as: 'name' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'amountCents', header: 'Amount', as: 'money' },
      { key: 'currency', header: 'Currency', as: 'text' },
      { key: 'createdAt', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'payouts',
    kind: '/v1/billing/payouts',
    label: 'Payouts',
    subtitle: 'Outbound disbursements and settlement transfers',
    empty: 'No payouts yet.',
    columns: [
      { key: 'id', header: 'Payout', as: 'name' },
      { key: 'destination', header: 'Destination', as: 'text' },
      { key: 'amountCents', header: 'Amount', as: 'money' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'createdAt', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'carts',
    kind: 'cart',
    label: 'Carts',
    subtitle: 'Active and abandoned shopper baskets for this organization',
    empty: 'No active or abandoned carts.',
    columns: [
      { key: 'id', header: 'Cart', as: 'name' },
      { key: 'email', header: 'Shopper', as: 'text' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'itemCount', header: 'Items', as: 'num' },
      { key: 'totalCents', header: 'Total', as: 'money' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'stores',
    kind: 'store',
    label: 'Stores',
    subtitle: 'Storefronts configured in this organization',
    empty: 'No stores configured yet.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'slug', header: 'Slug', as: 'text' },
      { key: 'domain', header: 'Domain', as: 'text' },
      { key: 'currency', header: 'Currency', as: 'text' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'products',
    kind: 'product',
    label: 'Products',
    subtitle: 'Manage your product catalog',
    empty: 'No products yet — create one to start selling.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'slug', header: 'Slug', as: 'text' },
      { key: 'price', header: 'Price', as: 'money' },
      { key: 'inventory', header: 'Inventory', as: 'num' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'collections',
    kind: 'collection',
    label: 'Collections',
    subtitle: 'Organize products into collections',
    empty: 'No collections yet.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'slug', header: 'Slug', as: 'text' },
      { key: 'published', header: 'State', as: 'flag', on: 'live', off: 'draft' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'discounts',
    kind: 'discount',
    label: 'Discounts',
    subtitle: 'Discount codes and promotional rules',
    empty: 'No discounts yet.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'code', header: 'Code', as: 'text' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'returns',
    kind: 'return',
    label: 'Returns',
    subtitle: 'Order returns and refund authorizations',
    empty: 'No returns.',
    columns: [
      { key: 'id', header: 'Return', as: 'name' },
      { key: 'orderId', header: 'Order', as: 'text' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'credits',
    kind: '/v1/billing/credits',
    label: 'Credits',
    subtitle: 'Credit grants, expirations, and promotional balances',
    empty: 'No credit grants yet.',
    columns: [
      { key: 'id', header: 'Grant', as: 'name' },
      { key: 'name', header: 'Name', as: 'text' },
      { key: 'amountCents', header: 'Amount', as: 'money' },
      { key: 'remainingCents', header: 'Remaining', as: 'money' },
      { key: 'expiresAt', header: 'Expires', as: 'date' },
    ],
  },
  {
    slug: 'usage',
    kind: '/v1/billing/usage',
    label: 'Usage',
    subtitle: 'Metered billable events attributed by product and model',
    empty: 'No metered usage recorded.',
    columns: [
      { key: 'transactionId', header: 'Event', as: 'name' },
      { key: 'description', header: 'Product', as: 'text' },
      { key: 'decimal', header: 'Quantity', as: 'text' },
      { key: 'amountCents', header: 'Amount', as: 'money' },
      { key: 'createdAt', header: 'Date', as: 'date' },
    ],
  },
  {
    slug: 'sales-channels',
    kind: 'saleschannel',
    label: 'Sales channels',
    subtitle: 'The storefronts and marketplaces where your products are sold',
    empty: 'No sales channels yet.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'description', header: 'Description', as: 'text' },
      { key: 'isDisabled', header: 'Status', as: 'flag', on: 'disabled', off: 'active' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
  {
    slug: 'stock-locations',
    kind: 'stocklocation',
    label: 'Stock locations',
    subtitle: 'The warehouses and stores that hold sellable inventory',
    empty: 'No stock locations yet.',
    columns: [
      { key: 'name', header: 'Name', as: 'name' },
      { key: 'city', header: 'City', as: 'text' },
      { key: 'country', header: 'Country', as: 'text' },
      { key: 'createdAt', header: 'Added', as: 'date' },
    ],
  },
  {
    slug: 'subscribers',
    kind: 'subscriber',
    label: 'Subscribers',
    subtitle: 'Storefront subscribers and mailing contacts',
    empty: 'No subscribers yet.',
    columns: [
      { key: 'email', header: 'Email', as: 'name' },
      { key: 'firstName', header: 'First name', as: 'text' },
      { key: 'lastName', header: 'Last name', as: 'text' },
      { key: 'createdAt', header: 'Joined', as: 'date' },
    ],
  },
  {
    slug: 'transfers',
    kind: 'transfer',
    label: 'Transfers',
    subtitle: 'Internal ledger and commerce transfers',
    empty: 'No transfers yet.',
    columns: [
      { key: 'id', header: 'Transfer', as: 'name' },
      { key: 'status', header: 'Status', as: 'status' },
      { key: 'amount', header: 'Amount', as: 'money' },
      { key: 'createdAt', header: 'Created', as: 'date' },
    ],
  },
]

export const resourceBySlug = (slug: string): Resource | undefined => RESOURCES.find((r) => r.slug === slug)
