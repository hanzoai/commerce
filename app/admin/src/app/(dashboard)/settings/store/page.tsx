'use client'

// Store details — the real form the old flat Settings page used to be, now a
// sub-section of the settings hub. Composes the ONE ResourceForm engine (schema +
// field list as data) so there is no bespoke form markup here.
import { useRouter } from 'next/navigation'
import { Button, Container, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { useStore, useUpdate } from '@/lib/api/hooks'
import { storeSchema, storeFields, storeDefaults, storePayload, type StoreRecord } from '@/components/settings/store-form'
import { errorMessage } from '@/lib/forms/schema'

export default function StoreSettingsPage() {
  const router = useRouter()
  const { data: store, isLoading } = useStore()
  const record = store as StoreRecord | null
  const update = useUpdate<StoreRecord>('store')

  return (
    <div>
      <ToasterMount />
      <PageHeader
        title="Store details"
        description="Name, currency, domain, and branding"
        actions={
          <Button type="button" size="small" variant="secondary" onClick={() => router.push('/settings')}>
            Back
          </Button>
        }
      />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          {isLoading ? (
            <div className="space-y-4">
              {[...Array(5)].map((_, i) => (
                <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
              ))}
            </div>
          ) : record ? (
            <ResourceForm
              schema={storeSchema}
              defaultValues={storeDefaults(record)}
              fields={storeFields}
              submitLabel="Save store"
              isPending={update.isPending}
              single
              onCancel={() => router.push('/settings')}
              onSubmit={async (values) => {
                try {
                  await update.mutateAsync({ id: record.id, data: storePayload(values, record) })
                  toast.success('Store settings saved')
                } catch (e) {
                  toast.error(errorMessage(e, 'Could not save store settings'))
                }
              }}
            />
          ) : (
            <p className="py-8 text-center text-sm text-ui-fg-muted">
              No store configuration found yet.
            </p>
          )}
        </Container>
      </div>
    </div>
  )
}
