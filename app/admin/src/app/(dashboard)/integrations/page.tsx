'use client'

import { Badge, Button, Container, Heading, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { useIntegrations } from '@/lib/api/hooks'

interface CatalogItem {
  type: string
  name: string
  group: string
  note: string
  builtIn?: boolean
}

const catalog: CatalogItem[] = [
  { type: 'square', name: 'Square', group: 'Payments', note: 'Cards, wallets, subscriptions, and checkout', builtIn: true },
  { type: 'stripe', name: 'Stripe', group: 'Payments', note: 'Card payments and Stripe Connect' },
  { type: 'paypal', name: 'PayPal', group: 'Payments', note: 'PayPal checkout and capture' },
  { type: 'authorizeNet', name: 'Authorize.net', group: 'Payments', note: 'Card authorization and capture' },
  { type: 'mailchimp', name: 'Mailchimp', group: 'Marketing', note: 'Customers, carts, and campaign audiences' },
  { type: 'sendgrid', name: 'SendGrid', group: 'Messaging', note: 'Transactional email delivery' },
  { type: 'smtprelay', name: 'SMTP', group: 'Messaging', note: 'Bring your own mail relay' },
  { type: 'analytics-google-analytics', name: 'Google Analytics', group: 'Analytics', note: 'Store and checkout analytics' },
  { type: 'analytics-facebook-pixel', name: 'Meta Pixel', group: 'Analytics', note: 'Ads and conversion measurement' },
  { type: 'analytics-sentry', name: 'Sentry', group: 'Operations', note: 'Storefront error monitoring' },
  { type: 'shipwire', name: 'Shipwire', group: 'Fulfillment', note: 'Warehousing and fulfillment' },
  { type: 'salesforce', name: 'Salesforce', group: 'CRM', note: 'Customer and sales data' },
]

export default function IntegrationsPage() {
  const { data = [], isLoading, save } = useIntegrations()

  return (
    <div>
      <PageHeader
        title="Integrations"
        description="Connect payments, fulfillment, marketing, analytics, and operations"
      />
      <div className="p-8">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {catalog.map((item) => {
            const integration = data.find((value) => value.type === item.type)
            const enabled = item.builtIn || !!integration?.enabled
            const configured = item.builtIn || !!integration

            return (
              <Container key={item.type} className="flex min-h-48 flex-col p-5">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <Text size="xsmall" className="text-ui-fg-muted">{item.group}</Text>
                    <Heading level="h3" className="mt-1">{item.name}</Heading>
                  </div>
                  <Badge color={enabled ? 'green' : configured ? 'orange' : 'grey'}>
                    {enabled ? 'Enabled' : configured ? 'Paused' : 'Available'}
                  </Badge>
                </div>
                <Text size="small" className="mt-3 flex-1 text-ui-fg-muted">{item.note}</Text>
                <div className="mt-5">
                  {item.builtIn ? (
                    <Text size="xsmall" className="text-ui-fg-muted">
                      Built in. Payment credentials are secured in Hanzo KMS.
                    </Text>
                  ) : integration ? (
                    <Button
                      size="small"
                      variant={enabled ? 'secondary' : 'primary'}
                      disabled={save.isPending}
                      onClick={() => save.mutate({ ...integration, enabled: !enabled })}
                    >
                      {enabled ? 'Pause' : 'Enable'}
                    </Button>
                  ) : (
                    <Button size="small" variant="secondary" disabled>
                      Add credentials
                    </Button>
                  )}
                </div>
              </Container>
            )
          })}
        </div>
        {!isLoading && data.length === 0 && (
          <Text size="xsmall" className="mt-5 text-ui-fg-muted">
            Provider secrets stay in Hanzo KMS. Add credentials there once, then enable the provider here.
          </Text>
        )}
      </div>
    </div>
  )
}
