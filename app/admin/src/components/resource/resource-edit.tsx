'use client'

// Generic detail + edit surface for a descriptor-driven resource. Reads the [id]
// route param, fetches the single record client-side, and renders the shared
// <ResourceFormLayout> (page chrome + sticky Save/Cancel + confirm-Delete) filled
// from the descriptor's field list. Save PATCHes, Delete removes — both toast and
// return to the list. The `extra` slot renders resource-specific panels (a
// category's children, an inventory item's levels, a reservation's adjust control)
// below the fields, receiving the loaded record. Every simple CRUD edit page is a
// one-line wrapper around this — no per-resource form markup.
import type { ReactNode } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Container, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import type { FieldValues } from 'react-hook-form'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { FieldRow } from '@/components/forms/resource-form/field-row'
import { useGet, useUpdate, useDelete } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import type { ResourceDescriptor } from './descriptor'

function paramId(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value
}

interface ResourceEditProps<T, V extends FieldValues> {
  descriptor: ResourceDescriptor<T, V>
  description?: string
  /** Resource-specific panels rendered below the general fields. */
  extra?: (record: T) => ReactNode
}

export function ResourceEdit<T extends { id: string }, V extends FieldValues>({
  descriptor,
  description,
  extra,
}: ResourceEditProps<T, V>) {
  const router = useRouter()
  const id = paramId(useParams()?.id as string | string[] | undefined)
  const label = descriptor.label.toLowerCase()

  const { data, isLoading } = useGet<T>(descriptor.kind, id)
  const update = useUpdate<T>(descriptor.kind)
  const remove = useDelete(descriptor.kind)

  const form = useForm<V>({
    values: data ? (descriptor.toValues(data) as never) : undefined,
    defaultValues: descriptor.empty as never,
    resolver: zodResolver(descriptor.schema as never),
  })

  if (isLoading || !id) {
    return (
      <div>
        <PageHeader title={descriptor.label} description="Loading…" />
        <div className="p-8">
          <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-10 w-full rounded-md" />
            ))}
          </Container>
        </div>
      </div>
    )
  }

  if (!data) {
    return (
      <div>
        <PageHeader title={descriptor.label} description={`This ${label} could not be found.`} />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push(descriptor.listPath)}
          >
            Back to list
          </Text>
        </div>
      </div>
    )
  }

  const title = descriptor.recordTitle ? descriptor.recordTitle(data) : descriptor.label

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      await update.mutateAsync({ id: data.id, data: descriptor.toPayload(values as V) })
      toast.success(`${descriptor.label} updated`)
      router.push(descriptor.listPath)
    } catch (e) {
      toast.error(errorMessage(e, `Could not update ${label}`))
    }
  })

  const onDelete = async () => {
    try {
      await remove.mutateAsync(data.id)
      toast.success(`${descriptor.label} deleted`)
      router.push(descriptor.listPath)
    } catch (e) {
      toast.error(errorMessage(e, `Could not delete ${label}`))
    }
  }

  return (
    <>
      <ToasterMount />
      <ResourceFormLayout
        title={title}
        description={description}
        backHref={descriptor.listPath}
        onSubmit={onSubmit}
        submitLabel="Save changes"
        submitting={update.isPending}
        onDelete={onDelete}
        deleting={remove.isPending}
        deleteLabel={descriptor.deleteLabel ?? `Delete ${label}`}
        deleteTitle={descriptor.deleteTitle ?? `Delete ${label}?`}
        deleteDescription={
          descriptor.deleteDescription ?? `This permanently removes the ${label} and cannot be undone.`
        }
      >
        <div className="flex flex-col gap-4">
          {descriptor.fields.map((spec) => (
            <FieldRow key={String(spec.name)} control={form.control} spec={spec} />
          ))}
        </div>
        {extra?.(data)}
      </ResourceFormLayout>
    </>
  )
}
