'use client'

// Create surface for a publishable API key. Unlike the generic create engine it
// must reveal the full token EXACTLY ONCE: the server returns it only on create and
// never again. On success we hold the created key and show a copy-once panel with
// a Done button, instead of redirecting immediately.
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Container, Text, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import { useCreate } from '@/lib/api/hooks'
import { apiKeyDescriptor, redactToken, type PublishableApiKey } from '@/lib/api-key'
import { errorMessage } from '@/lib/forms/schema'

export function ApiKeyCreate() {
  const router = useRouter()
  const [created, setCreated] = useState<PublishableApiKey | null>(null)
  const { mutateAsync, isPending } = useCreate<PublishableApiKey>('publishableapikey')

  const copy = async (token: string) => {
    try {
      await navigator.clipboard.writeText(token)
      toast.success('Token copied')
    } catch {
      toast.error('Could not copy — select and copy it manually')
    }
  }

  return (
    <div>
      <ToasterMount />
      <PageHeader
        title="Create API key"
        description="Publishable keys authorize your storefront to read the catalog."
      />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          {created ? (
            <div className="flex flex-col gap-y-6">
              <Section title="Copy your key now">
                <div className="px-6 py-4">
                  <Text size="small" className="text-ui-fg-subtle">
                    This is the only time the full token is shown. Store it somewhere safe.
                  </Text>
                </div>
                <SectionRow
                  title="Token"
                  value={
                    <Text family="mono" size="small" className="break-all">
                      {created.token || redactToken(created.redacted) || '—'}
                    </Text>
                  }
                  actions={
                    created.token ? (
                      <Button size="small" variant="secondary" onClick={() => copy(created.token as string)}>
                        Copy
                      </Button>
                    ) : undefined
                  }
                />
              </Section>
              <div className="flex justify-end">
                <Button size="small" variant="primary" onClick={() => router.push('/api-keys')}>
                  Done
                </Button>
              </div>
            </div>
          ) : (
            <ResourceForm
              schema={apiKeyDescriptor.schema}
              defaultValues={apiKeyDescriptor.empty}
              fields={apiKeyDescriptor.fields}
              submitLabel="Create"
              isPending={isPending}
              single
              onCancel={() => router.push('/api-keys')}
              onSubmit={async (values) => {
                try {
                  const key = await mutateAsync(apiKeyDescriptor.toPayload(values))
                  toast.success('API key created')
                  setCreated(key)
                } catch (e) {
                  toast.error(errorMessage(e, 'Could not create API key'))
                }
              }}
            />
          )}
        </Container>
      </div>
    </div>
  )
}
