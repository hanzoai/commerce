/**
 * The admin's resource catalog — DATA, and only data.
 *
 * Every merchant surface is the same thing: a `/v1/<kind>` list rendered by the ONE
 * shared `CommerceResource` from `@hanzo/ui/product`. So a page is not a file of
 * table plumbing, it is a ROW here: slug, backend noun, copy, columns. Adding a
 * surface means adding a row — the sidebar, the static route and the page all read
 * this list.
 *
 * Deliberately free of any component import: `generateStaticParams` runs this module
 * in Node at build time. `~/lib/columns` turns a `ColumnSpec` into a rendered column.
 */

/** Any row the API returns — resources are described by columns, not by types. */
export type Row = Record<string, unknown>

/** How a cell reads: the emphasised first column, muted text, a date, money (minor
 *  units in the row's own currency), a count, a status tag, or a boolean-as-state. */
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
  /** The backend noun: `/v1/<kind>`. */
  kind: string
  label: string
  subtitle: string
  /** Honest empty copy for a store that has none of these yet. */
  empty: string
  columns: ColumnSpec[]
}

export const RESOURCES: Resource[] = [
  {
    slug: 'products',
    kind: 'product',
    label: 'Products',
    subtitle: 'Manage your product catalog',
    empty: 'No products yet — create one to start selling.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'slug', header: 'Slug', as: 'text' }, { key: 'price', header: 'Price', as: 'money' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'orders',
    kind: 'order',
    label: 'Orders',
    subtitle: 'View and manage customer orders',
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
    slug: 'customers',
    kind: 'c/user',
    label: 'Customers',
    subtitle: 'View and manage customer accounts',
    empty: 'No customers yet.',
    columns: [
      { key: 'email', header: 'Email', as: 'name' },
      { key: 'firstName', header: 'First name', as: 'text' },
      { key: 'lastName', header: 'Last name', as: 'text' },
      { key: 'orderCount', header: 'Orders', as: 'num' },
      { key: 'createdAt', header: 'Joined', as: 'date' },
    ],
  },
  {
    slug: 'collections',
    kind: 'collection',
    label: 'Collections',
    subtitle: 'Organize products into collections',
    empty: 'No collections yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'slug', header: 'Slug', as: 'text' }, { key: 'published', header: 'State', as: 'flag', on: 'live', off: 'draft' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'categories',
    kind: 'category',
    label: 'Categories',
    subtitle: 'Nestable groupings that organize your product catalog',
    empty: 'No categories yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'handle', header: 'Handle', as: 'text' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'types',
    kind: 'producttype',
    label: 'Types',
    subtitle: 'Classify products by type for reporting and organization',
    empty: 'No product types yet.',
    columns: [{ key: 'value', header: 'Value', as: 'name' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'inventory-items',
    kind: 'inventoryitem',
    label: 'Inventory items',
    subtitle: 'The stock-keeping units tracked across your locations',
    empty: 'No inventory items yet.',
    columns: [{ key: 'sku', header: 'SKU', as: 'name' }, { key: 'title', header: 'Title', as: 'text' }, { key: 'requiresShipping', header: 'Shipping', as: 'flag', on: 'active', off: 'none' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'stock-locations',
    kind: 'stocklocation',
    label: 'Stock locations',
    subtitle: 'The warehouses and stores that hold sellable inventory',
    empty: 'No stock locations yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'city', header: 'City', as: 'text' }, { key: 'country', header: 'Country', as: 'text' }, { key: 'createdAt', header: 'Added', as: 'date' }],
  },
  {
    slug: 'reservations',
    kind: 'reservation',
    label: 'Reservations',
    subtitle: 'Stock held against open orders, by item and location',
    empty: 'No reservations.',
    columns: [{ key: 'inventoryItemId', header: 'Inventory item', as: 'text' }, { key: 'locationId', header: 'Location', as: 'text' }, { key: 'quantity', header: 'Quantity', as: 'num' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'promotions',
    kind: 'promotion',
    label: 'Promotions',
    subtitle: 'Automatic and code-based promotions',
    empty: 'No promotions yet.',
    columns: [{ key: 'code', header: 'Code', as: 'name' }, { key: 'type', header: 'Type', as: 'text' }, { key: 'status', header: 'Status', as: 'status' }, { key: 'isAutomatic', header: 'Automatic', as: 'flag', on: 'active', off: 'manual' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'price-lists',
    kind: 'pricelist',
    label: 'Price lists',
    subtitle: 'Sale and override pricing, plus store-wide price preferences',
    empty: 'No price lists yet.',
    columns: [{ key: 'title', header: 'Title', as: 'name' }, { key: 'type', header: 'Type', as: 'text' }, { key: 'status', header: 'Status', as: 'status' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'regions',
    kind: 'region',
    label: 'Regions',
    subtitle: 'Market regions, their currency, and the countries they cover',
    empty: 'No regions yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'currencyCode', header: 'Currency', as: 'text' }, { key: 'automaticTaxes', header: 'Auto taxes', as: 'flag', on: 'active', off: 'off' }],
  },
  {
    slug: 'tax-regions',
    kind: 'taxregion',
    label: 'Tax regions',
    subtitle: 'Configure tax regions and their rates',
    empty: 'No tax regions yet.',
    columns: [{ key: 'countryCode', header: 'Country', as: 'name' }, { key: 'provinceCode', header: 'Province', as: 'text' }, { key: 'providerId', header: 'Provider', as: 'text' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'currencies',
    kind: 'currency',
    label: 'Currencies',
    subtitle: 'Enable or disable which currencies your store accepts',
    empty: 'No currencies enabled.',
    columns: [{ key: 'code', header: 'Code', as: 'name' }, { key: 'name', header: 'Name', as: 'text' }, { key: 'symbol', header: 'Symbol', as: 'text' }, { key: 'decimalDigits', header: 'Decimals', as: 'num' }],
  },
  {
    slug: 'sales-channels',
    kind: 'saleschannel',
    label: 'Sales channels',
    subtitle: 'The storefronts and marketplaces where your products are sold',
    empty: 'No sales channels yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'description', header: 'Description', as: 'text' }, { key: 'isDisabled', header: 'Status', as: 'flag', on: 'disabled', off: 'active' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'customer-groups',
    kind: 'customergroup',
    label: 'Customer groups',
    subtitle: 'Group customers for segments, targeting and pricing',
    empty: 'No customer groups yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'claims',
    kind: 'claim',
    label: 'Claims',
    subtitle: 'Resolve damaged, wrong, or missing items with a refund or replacement',
    empty: 'No claims.',
    columns: [{ key: 'orderId', header: 'Order', as: 'name' }, { key: 'resolution', header: 'Resolution', as: 'text' }, { key: 'amountCents', header: 'Amount', as: 'money' }, { key: 'status', header: 'Status', as: 'status' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'api-keys',
    kind: 'publishableapikey',
    label: 'API keys',
    subtitle: 'Publishable keys that authorize your storefront to read the catalog',
    empty: 'No API keys yet.',
    columns: [{ key: 'title', header: 'Title', as: 'name' }, { key: 'redacted', header: 'Token', as: 'text' }, { key: 'revokedAt', header: 'Status', as: 'flag', on: 'revoked', off: 'active' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
  {
    slug: 'roles',
    kind: 'role',
    label: 'Roles',
    subtitle: 'Permission groups you can assign to team members',
    empty: 'No roles yet.',
    columns: [{ key: 'name', header: 'Name', as: 'name' }, { key: 'description', header: 'Description', as: 'text' }, { key: 'createdAt', header: 'Created', as: 'date' }],
  },
]

export const resourceBySlug = (slug: string): Resource | undefined => RESOURCES.find((r) => r.slug === slug)
