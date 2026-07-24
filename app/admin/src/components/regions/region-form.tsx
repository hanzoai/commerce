'use client'

// The ONE region form. Both the create page and the detail/edit view render
// through it, driven by the shared schema + mappers. Chrome from
// <ResourceFormLayout>; inputs from the reusable field primitives.

import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { TextField } from '@/components/common/form-fields'
import { SelectField, SwitchField } from '@/components/common/choice-fields'
import { regionSchema, CURRENCY_OPTIONS, type RegionValues } from '@/lib/regions/region'

interface RegionFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: RegionValues
  submitting?: boolean
  onSubmit: (values: RegionValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  extra?: ReactNode
}

export function RegionForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  extra,
}: RegionFormProps) {
  const { control, handleSubmit } = useForm<RegionValues>({
    defaultValues,
    resolver: zodResolver(regionSchema),
  })

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/regions"
      onSubmit={handleSubmit((values) => onSubmit(values))}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete region"
      deleteTitle="Delete region?"
      deleteDescription="This permanently removes the region and its country assignments. This cannot be undone."
    >
      <div className="flex flex-col gap-y-6">
        <TextField control={control} name="name" label="Name" placeholder="North America" autoFocus />
        <SelectField
          control={control}
          name="currencyCode"
          label="Currency"
          options={CURRENCY_OPTIONS}
          placeholder="Select currency"
          className="sm:max-w-sm"
        />
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SwitchField
            control={control}
            name="automaticTaxes"
            label="Automatic taxes"
            description="Compute taxes automatically for this region."
          />
          <SwitchField
            control={control}
            name="taxInclusiveEnabled"
            label="Tax-inclusive pricing"
            description="Prices in this region include tax."
          />
        </div>
      </div>

      {extra}
    </ResourceFormLayout>
  )
}
