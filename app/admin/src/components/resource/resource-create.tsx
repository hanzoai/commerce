'use client'

// Generic create surface for a descriptor-driven resource. It is the customer-
// groups create pattern (PageHeader + centered Container + the shared field-driven
// <ResourceForm>) lifted into one reusable engine, so a new resource's create page
// is a three-line wrapper around its descriptor. On success it toasts and returns
// to the list; the freshly-created record's id is handed to `onCreated` for
// surfaces (api-keys) that must reveal a one-time secret before redirecting.
import { useRouter } from 'next/navigation'
import { Container, toast } from '@hanzo/commerce-ui'
import type { FieldValues } from 'react-hook-form'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { useCreate } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import type { ResourceDescriptor } from './descriptor'

interface ResourceCreateProps<T, V extends FieldValues> {
  descriptor: ResourceDescriptor<T, V>
  title?: string
  description?: string
  submitLabel?: string
  /** Runs after a successful create — return false to keep the page (e.g. to
   *  reveal a one-time token); anything else redirects to the list. */
  onCreated?: (record: T) => void | boolean | Promise<void | boolean>
}

export function ResourceCreate<T, V extends FieldValues>({
  descriptor,
  title,
  description,
  submitLabel = 'Create',
  onCreated,
}: ResourceCreateProps<T, V>) {
  const router = useRouter()
  const { mutateAsync, isPending } = useCreate<T>(descriptor.kind)
  const label = descriptor.label.toLowerCase()

  return (
    <div>
      <ToasterMount />
      <PageHeader title={title ?? `Create ${label}`} description={description} />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          <ResourceForm
            schema={descriptor.schema}
            defaultValues={descriptor.empty as never}
            fields={descriptor.fields}
            submitLabel={submitLabel}
            isPending={isPending}
            single
            onCancel={() => router.push(descriptor.listPath)}
            onSubmit={async (values) => {
              try {
                const record = await mutateAsync(descriptor.toPayload(values as V))
                toast.success(`${descriptor.label} created`)
                const keep = onCreated ? await onCreated(record) : undefined
                if (keep !== false) router.push(descriptor.listPath)
              } catch (e) {
                toast.error(errorMessage(e, `Could not create ${label}`))
              }
            }}
          />
        </Container>
      </div>
    </div>
  )
}
