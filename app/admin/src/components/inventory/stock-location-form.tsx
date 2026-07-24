'use client'

// The ONE stock-location form. Both the create page and the edit page render
// through it — driven by the shared schema + field list from the domain module —
// so the two surfaces can never drift. Layout/save/cancel/delete chrome comes
// from <ResourceForm>; each input from the reusable <TextField>.

import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { TextField } from '@/components/common/form-fields'
import {
  stockLocationFields,
  stockLocationSchema,
  type StockLocationValues,
} from '@/lib/inventory/stock-location'

interface StockLocationFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: StockLocationValues
  submitting?: boolean
  onSubmit: (values: StockLocationValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  /** Optional read-only summary rendered above the fields (edit view). */
  header?: ReactNode
}

export function StockLocationForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  header,
}: StockLocationFormProps) {
  const form = useForm<StockLocationValues>({
    defaultValues,
    resolver: zodResolver(stockLocationSchema),
  })

  const handleSubmit = form.handleSubmit((values) => onSubmit(values))

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/inventory"
      onSubmit={handleSubmit}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete location"
      deleteTitle="Delete stock location?"
      deleteDescription="This permanently removes the location. Inventory levels tracked against it will no longer be available."
    >
      {header}
      <div className="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
        {stockLocationFields.map((field) => (
          <TextField
            key={field.name}
            control={form.control}
            name={field.name}
            label={field.label}
            optional={field.optional}
            placeholder={field.placeholder}
            autoFocus={field.name === 'name'}
            className={field.span ? 'sm:col-span-2' : undefined}
          />
        ))}
      </div>
    </ResourceFormLayout>
  )
}
