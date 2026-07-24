'use client'

// The ONE price-list form. Both the create page and the detail/edit view render
// through it, driven by the shared schema + mappers — so the two surfaces can
// never drift. Chrome from <ResourceFormLayout>; inputs from the reusable field
// primitives.

import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { TextField, TextareaField } from '@/components/common/form-fields'
import { SelectField } from '@/components/common/choice-fields'
import {
  priceListSchema,
  STATUS_OPTIONS,
  TYPE_OPTIONS,
  type PriceListValues,
} from '@/lib/price-lists/price-list'

interface PriceListFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: PriceListValues
  submitting?: boolean
  onSubmit: (values: PriceListValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  extra?: ReactNode
}

export function PriceListForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  extra,
}: PriceListFormProps) {
  const { control, handleSubmit } = useForm<PriceListValues>({
    defaultValues,
    resolver: zodResolver(priceListSchema),
  })

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/price-lists"
      onSubmit={handleSubmit((values) => onSubmit(values))}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete price list"
      deleteTitle="Delete price list?"
      deleteDescription="This permanently removes the price list and its prices. This cannot be undone."
    >
      <div className="flex flex-col gap-y-6">
        <TextField control={control} name="title" label="Title" placeholder="Black Friday" autoFocus />
        <TextareaField control={control} name="description" label="Description" optional placeholder="What is this price list for?" />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField control={control} name="status" label="Status" options={STATUS_OPTIONS} placeholder="Select status" />
          <SelectField control={control} name="type" label="Type" options={TYPE_OPTIONS} placeholder="Select type" />
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField control={control} name="startsAt" label="Starts at" type="date" optional />
          <TextField control={control} name="endsAt" label="Ends at" type="date" optional />
        </div>
      </div>

      {extra}
    </ResourceFormLayout>
  )
}
