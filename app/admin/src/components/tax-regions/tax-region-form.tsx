'use client'

// The ONE tax-region form. Both the create page and the detail/edit view render
// through it, driven by the shared schema + mappers. Chrome from
// <ResourceFormLayout>; inputs from the reusable field primitives.

import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { TextField } from '@/components/common/form-fields'
import { taxRegionSchema, type TaxRegionValues } from '@/lib/tax-regions/tax-region'

interface TaxRegionFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: TaxRegionValues
  submitting?: boolean
  onSubmit: (values: TaxRegionValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  extra?: ReactNode
}

export function TaxRegionForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  extra,
}: TaxRegionFormProps) {
  const { control, handleSubmit } = useForm<TaxRegionValues>({
    defaultValues,
    resolver: zodResolver(taxRegionSchema),
  })

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/tax-regions"
      onSubmit={handleSubmit((values) => onSubmit(values))}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete tax region"
      deleteTitle="Delete tax region?"
      deleteDescription="This permanently removes the tax region and its rates. This cannot be undone."
    >
      <div className="flex flex-col gap-y-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField
            control={control}
            name="countryCode"
            label="Country code"
            placeholder="us"
            hint="2-letter ISO country code."
            autoFocus
          />
          <TextField
            control={control}
            name="provinceCode"
            label="Province / state code"
            optional
            placeholder="ca"
            hint="Leave blank for a country-level region."
          />
        </div>
        <TextField
          control={control}
          name="providerId"
          label="Tax provider ID"
          optional
          placeholder="system"
          hint="The tax provider that computes rates for this region."
        />
      </div>

      {extra}
    </ResourceFormLayout>
  )
}
