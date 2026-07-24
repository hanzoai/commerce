// The Hanzo Commerce integrations marketplace catalog. Each provider declares
// the credential fields its Configure form renders; on save those values become
// the integration's `data`, which the commerce API hydrates into Hanzo KMS at
// /tenants/{org}/{type}/* — the row itself never stores raw secrets.

export type CredKind = 'text' | 'password' | 'select'

export interface CredField {
  key: string
  label: string
  kind?: CredKind
  placeholder?: string
  required?: boolean
  secret?: boolean
  help?: string
  options?: { value: string; label: string }[]
}

export interface Provider {
  type: string
  name: string
  group: string
  note: string
  emoji: string
  /** Platform-managed default (creds already live in Hanzo KMS). */
  managed?: boolean
  docs?: string
  fields: CredField[]
}

const environment: CredField = {
  key: 'environment',
  label: 'Environment',
  kind: 'select',
  required: true,
  options: [
    { value: 'sandbox', label: 'Sandbox' },
    { value: 'production', label: 'Production' },
  ],
}

// Payments lead — Shopify-style, one-click enable per provider.
export const catalog: Provider[] = [
  {
    type: 'square',
    name: 'Square',
    group: 'Payments',
    emoji: '⬛️',
    note: 'Cards, wallets, subscriptions, and hosted checkout.',
    managed: true,
    fields: [
      { key: 'applicationId', label: 'Application ID', required: true, placeholder: 'sq0idp-…' },
      { key: 'accessToken', label: 'Access Token', secret: true, required: true },
      { key: 'locationId', label: 'Location ID', required: true },
      environment,
    ],
  },
  {
    type: 'stripe',
    name: 'Stripe',
    group: 'Payments',
    emoji: '💳',
    note: 'Card payments, wallets, and Stripe Connect.',
    fields: [
      { key: 'publishableKey', label: 'Publishable Key', required: true, placeholder: 'pk_live_…' },
      { key: 'secretKey', label: 'Secret Key', secret: true, required: true, placeholder: 'sk_live_…' },
      { key: 'webhookSecret', label: 'Webhook Signing Secret', secret: true, placeholder: 'whsec_…' },
    ],
  },
  {
    type: 'paypal',
    name: 'PayPal',
    group: 'Payments',
    emoji: '🅿️',
    note: 'PayPal checkout, capture, and refunds.',
    fields: [
      { key: 'clientId', label: 'Client ID', required: true },
      { key: 'clientSecret', label: 'Client Secret', secret: true, required: true },
      environment,
    ],
  },
  {
    type: 'authorizeNet',
    name: 'Authorize.net',
    group: 'Payments',
    emoji: '🏦',
    note: 'Card authorization and capture.',
    fields: [
      { key: 'apiLoginId', label: 'API Login ID', required: true },
      { key: 'transactionKey', label: 'Transaction Key', secret: true, required: true },
      environment,
    ],
  },
  {
    type: 'mailchimp',
    name: 'Mailchimp',
    group: 'Marketing',
    emoji: '🐵',
    note: 'Sync customers, carts, and campaign audiences.',
    fields: [
      { key: 'apiKey', label: 'API Key', secret: true, required: true },
      { key: 'serverPrefix', label: 'Server Prefix', required: true, placeholder: 'us21', help: 'The datacenter suffix on your API key.' },
      { key: 'audienceId', label: 'Audience ID' },
    ],
  },
  {
    type: 'sendgrid',
    name: 'SendGrid',
    group: 'Messaging',
    emoji: '📨',
    note: 'Transactional email delivery.',
    fields: [
      { key: 'apiKey', label: 'API Key', secret: true, required: true, placeholder: 'SG.…' },
      { key: 'fromEmail', label: 'From Address', required: true, placeholder: 'store@example.com' },
    ],
  },
  {
    type: 'smtprelay',
    name: 'SMTP',
    group: 'Messaging',
    emoji: '✉️',
    note: 'Bring your own mail relay.',
    fields: [
      { key: 'host', label: 'Host', required: true, placeholder: 'smtp.example.com' },
      { key: 'port', label: 'Port', required: true, placeholder: '587' },
      { key: 'username', label: 'Username' },
      { key: 'password', label: 'Password', secret: true },
      { key: 'fromEmail', label: 'From Address', required: true },
    ],
  },
  {
    type: 'analytics-google-analytics',
    name: 'Google Analytics',
    group: 'Analytics',
    emoji: '📊',
    note: 'Store and checkout analytics.',
    fields: [
      { key: 'measurementId', label: 'Measurement ID', required: true, placeholder: 'G-XXXXXXXXXX' },
    ],
  },
  {
    type: 'analytics-facebook-pixel',
    name: 'Meta Pixel',
    group: 'Analytics',
    emoji: '📈',
    note: 'Ads and conversion measurement.',
    fields: [
      { key: 'pixelId', label: 'Pixel ID', required: true },
    ],
  },
  {
    type: 'analytics-sentry',
    name: 'Sentry',
    group: 'Operations',
    emoji: '🛡️',
    note: 'Storefront error monitoring.',
    fields: [
      { key: 'dsn', label: 'DSN', required: true, placeholder: 'https://…@sentry.io/…' },
    ],
  },
  {
    type: 'shipwire',
    name: 'Shipwire',
    group: 'Fulfillment',
    emoji: '📦',
    note: 'Warehousing and fulfillment.',
    fields: [
      { key: 'username', label: 'Username', required: true },
      { key: 'password', label: 'Password', secret: true, required: true },
      environment,
    ],
  },
  {
    type: 'salesforce',
    name: 'Salesforce',
    group: 'CRM',
    emoji: '☁️',
    note: 'Customer and sales data.',
    fields: [
      { key: 'clientId', label: 'Consumer Key', required: true },
      { key: 'clientSecret', label: 'Consumer Secret', secret: true, required: true },
      { key: 'instanceUrl', label: 'Instance URL', required: true, placeholder: 'https://your.my.salesforce.com' },
    ],
  },
]

export const groups: string[] = Array.from(new Set(catalog.map((p) => p.group)))

export function providerByType(type: string): Provider | undefined {
  return catalog.find((p) => p.type === type)
}
