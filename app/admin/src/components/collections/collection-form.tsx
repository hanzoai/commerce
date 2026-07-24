'use client'

// The ONE collection form. Both the create page and the edit/detail view render
// through it — driven by the shared schema + mappers from the domain module — so
// the two surfaces can never drift. Page chrome (header, back, save, cancel,
// confirm-delete) comes from <ResourceForm>; each input from the reusable field
// primitives. On create the handle auto-derives from the name until the user
// edits it directly.

import { useEffect, useRef, type ReactNode } from 'react'
import {
  Controller,
  useForm,
  type Control,
  type FieldPath,
  type FieldValues,
} from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Input, Switch, Text } from '@hanzo/commerce-ui'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { FieldRow, TextField, TextareaField } from '@/components/common/form-fields'
import { collectionSchema, slugify, type CollectionValues } from '@/lib/collections/collection'

interface CollectionFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: CollectionValues
  submitting?: boolean
  onSubmit: (values: CollectionValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  /** Derive the handle from the name until the user edits it (create surface). */
  autoSlug?: boolean
  /** Extra sections rendered below the fields (e.g. product assignment on edit). */
  extra?: ReactNode
}

export function CollectionForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  autoSlug,
  extra,
}: CollectionFormProps) {
  const form = useForm<CollectionValues>({
    defaultValues,
    resolver: zodResolver(collectionSchema),
  })
  const { control, handleSubmit, watch, setValue } = form

  // Handle auto-derivation: only while the user has not typed into the handle.
  const slugTouched = useRef(!autoSlug)
  const name = watch('name')
  useEffect(() => {
    if (!autoSlug || slugTouched.current) return
    setValue('slug', slugify(name), { shouldValidate: true })
  }, [name, autoSlug, setValue])

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/collections"
      onSubmit={handleSubmit((values) => onSubmit(values))}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete collection"
      deleteTitle="Delete collection?"
      deleteDescription="This permanently removes the collection. Products stay in your catalog — they are just no longer grouped here."
    >
      <div className="flex flex-col gap-y-6">
        <TextField
          control={control}
          name="name"
          label="Name"
          placeholder="Summer Essentials"
          autoFocus
        />

        <Controller
          control={control}
          name="slug"
          render={({ field, fieldState }) => (
            <FieldRow
              id="collection-handle"
              label="Handle"
              hint="The url-safe identifier used in your storefront."
              error={fieldState.error?.message}
            >
              <div className="relative">
                <div className="absolute inset-y-0 left-0 z-10 flex w-8 items-center justify-center border-r border-ui-border-base">
                  <Text size="small" leading="compact" className="text-ui-fg-muted">
                    /
                  </Text>
                </div>
                <Input
                  id="collection-handle"
                  {...field}
                  value={field.value ?? ''}
                  placeholder="summer-essentials"
                  className="pl-10"
                  onChange={(event) => {
                    slugTouched.current = true
                    field.onChange(event)
                  }}
                />
              </div>
            </FieldRow>
          )}
        />

        <TextareaField
          control={control}
          name="description"
          label="Description"
          optional
          placeholder="What ties these products together?"
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ToggleField
            control={control}
            name="published"
            label="Published"
            hint="Show this collection in your storefront."
          />
          <ToggleField
            control={control}
            name="available"
            label="Available"
            hint="Allow its products to be purchased."
          />
        </div>
      </div>

      {extra}
    </ResourceFormLayout>
  )
}

interface ToggleFieldProps<T extends FieldValues> {
  control: Control<T>
  name: FieldPath<T>
  label: string
  hint?: string
}

/** A labelled boolean toggle bound to a react-hook-form control. */
function ToggleField<T extends FieldValues>({ control, name, label, hint }: ToggleFieldProps<T>) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <label className="flex items-start justify-between gap-x-3 rounded-lg border border-ui-border-base px-4 py-3">
          <div className="min-w-0">
            <Text size="small" weight="plus" className="text-ui-fg-base">
              {label}
            </Text>
            {hint && (
              <Text size="xsmall" leading="compact" className="mt-0.5 text-ui-fg-subtle">
                {hint}
              </Text>
            )}
          </div>
          <Switch checked={Boolean(field.value)} onCheckedChange={field.onChange} />
        </label>
      )}
    />
  )
}
