'use client'

import { Heading, Text, Container } from '@hanzo/commerce-ui'
import { useStore } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'

export default function SettingsPage() {
  const { data: store, isLoading } = useStore()

  return (
    <div>
      <PageHeader title="Settings" description="Store configuration and preferences" />
      <div className="p-8">
        <Container className="p-6">
          <Heading level="h3" className="mb-4">Store Details</Heading>
          {isLoading ? (
            <div className="space-y-4">
              {[...Array(4)].map((_, i) => (
                <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
              ))}
            </div>
          ) : store ? (
            <dl className="space-y-4">
              <Field label="Store Name" value={store.name} />
              <Field label="Currency" value={store.defaultCurrency || store.currency} />
              <Field label="Default Region" value={store.defaultRegion || store.region} />
              <Field label="Created" value={store.createdAt ? new Date(store.createdAt).toLocaleString() : undefined} />
            </dl>
          ) : (
            <Text size="small" className="py-8 text-center text-ui-fg-muted">
              No store configuration found. The API may not be configured yet.
            </Text>
          )}
        </Container>

        <Container className="mt-6 p-6">
          <Heading level="h3" className="mb-4">API Configuration</Heading>
          <dl className="space-y-4">
            <Field
              label="API Endpoint"
              value={process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://commerce-api.hanzo.ai'}
            />
            <Field
              label="IAM Server"
              value={process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'}
            />
          </dl>
        </Container>
      </div>
    </div>
  )
}

function Field({ label, value }: { label: string; value?: string | null }) {
  return (
    <div>
      <Text as="span" size="xsmall" className="text-ui-fg-muted">{label}</Text>
      <Text size="small" className="mt-0.5 text-ui-fg-base">{value || '-'}</Text>
    </div>
  )
}
