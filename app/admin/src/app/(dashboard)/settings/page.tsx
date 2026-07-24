'use client'

import { useEffect, useState } from 'react'
import { Button, Heading, Input, Text, Container, toast } from '@hanzo/commerce-ui'
import { useStore, useUpdate } from '@/lib/api/hooks'
import type { CurrentStore } from '@/lib/api/data-provider'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { errorMessage } from '@/lib/forms/schema'

export default function SettingsPage() {
  const { data: store, isLoading } = useStore()
  const update = useUpdate<CurrentStore>('store')
  const [name, setName] = useState('')
  const [currency, setCurrency] = useState('usd')

  useEffect(() => {
    if (!store) return
    setName(store.name || '')
    setCurrency(store.currency || store.defaultCurrency || 'usd')
  }, [store])

  const save = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!store) return
    try {
      await update.mutateAsync({
        id: store.id,
        data: { name: name.trim(), currency: currency.trim().toLowerCase() },
      })
      toast.success('Store settings saved')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not save store settings'))
    }
  }

  return (
    <div>
      <ToasterMount />
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
            <form className="max-w-xl space-y-4" onSubmit={save}>
              <label className="block">
                <Text as="span" size="xsmall" className="text-ui-fg-muted">Store name</Text>
                <Input className="mt-1.5" value={name} onChange={(event) => setName(event.target.value)} />
              </label>
              <label className="block">
                <Text as="span" size="xsmall" className="text-ui-fg-muted">Currency</Text>
                <Input className="mt-1.5" value={currency} onChange={(event) => setCurrency(event.target.value)} maxLength={3} />
              </label>
              <Field label="Default Region" value={store.defaultRegion || store.region} />
              <Field label="Created" value={store.createdAt ? new Date(store.createdAt).toLocaleString() : undefined} />
              <Button size="small" type="submit" disabled={!name.trim() || update.isPending}>
                {update.isPending ? 'Saving…' : 'Save store'}
              </Button>
            </form>
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
              value={process.env.NEXT_PUBLIC_COMMERCE_API_URL || 'https://commerce.hanzo.ai'}
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
