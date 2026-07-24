'use client'

// Notification preferences — a settings sub-section. Composes the ONE ResourceForm
// engine with switch fields; toggles persist in the store row's metadata.
import { useRouter } from 'next/navigation'
import { Button, Container, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { useStore, useUpdate } from '@/lib/api/hooks'
import {
  notificationsSchema,
  notificationsFields,
  notificationsDefaults,
  notificationsPayload,
} from '@/components/settings/notifications-form'
import type { StoreRecord } from '@/components/settings/store-form'
import { errorMessage } from '@/lib/forms/schema'

export default function NotificationsSettingsPage() {
  const router = useRouter()
  const { data: store, isLoading } = useStore()
  const record = store as StoreRecord | null
  const update = useUpdate<StoreRecord>('store')

  return (
    <div>
      <ToasterMount />
      <PageHeader
        title="Notifications"
        description="Choose which events email you"
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
              {[...Array(4)].map((_, i) => (
                <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
              ))}
            </div>
          ) : record ? (
            <ResourceForm
              schema={notificationsSchema}
              defaultValues={notificationsDefaults(record)}
              fields={notificationsFields}
              submitLabel="Save preferences"
              isPending={update.isPending}
              single
              onCancel={() => router.push('/settings')}
              onSubmit={async (values) => {
                try {
                  await update.mutateAsync({ id: record.id, data: notificationsPayload(values, record) })
                  toast.success('Notification preferences saved')
                } catch (e) {
                  toast.error(errorMessage(e, 'Could not save preferences'))
                }
              }}
            />
          ) : (
            <p className="py-8 text-center text-sm text-ui-fg-muted">
              Set up your store first to manage notifications.
            </p>
          )}
        </Container>
      </div>
    </div>
  )
}
