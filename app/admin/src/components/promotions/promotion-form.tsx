'use client'

// The ONE promotion form. Both the create page and the detail/edit view render
// through it, driven by the shared schema + mappers — so the two surfaces can
// never drift. Page chrome (header, back, save, cancel, confirm-delete) comes
// from <ResourceFormLayout>; each input from the reusable field primitives.

import type { ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { ResourceFormLayout } from '@/components/common/resource-form'
import { TextField } from '@/components/common/form-fields'
import { SelectField, SwitchField } from '@/components/common/choice-fields'
import {
  promotionSchema,
  STATUS_OPTIONS,
  TYPE_OPTIONS,
  type PromotionValues,
} from '@/lib/promotions/promotion'

interface PromotionFormProps {
  title: string
  description?: string
  submitLabel: string
  defaultValues: PromotionValues
  submitting?: boolean
  onSubmit: (values: PromotionValues) => void | Promise<void>
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  extra?: ReactNode
}

export function PromotionForm({
  title,
  description,
  submitLabel,
  defaultValues,
  submitting,
  onSubmit,
  onDelete,
  deleting,
  extra,
}: PromotionFormProps) {
  const { control, handleSubmit } = useForm<PromotionValues>({
    defaultValues,
    resolver: zodResolver(promotionSchema),
  })

  return (
    <ResourceFormLayout
      title={title}
      description={description}
      backHref="/promotions"
      onSubmit={handleSubmit((values) => onSubmit(values))}
      submitLabel={submitLabel}
      submitting={submitting}
      onDelete={onDelete}
      deleting={deleting}
      deleteLabel="Delete promotion"
      deleteTitle="Delete promotion?"
      deleteDescription="This permanently removes the promotion and its application method. This cannot be undone."
    >
      <div className="flex flex-col gap-y-6">
        <TextField control={control} name="code" label="Code" placeholder="SUMMER15" autoFocus />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField control={control} name="status" label="Status" options={STATUS_OPTIONS} placeholder="Select status" />
          <SelectField control={control} name="type" label="Type" options={TYPE_OPTIONS} placeholder="Select type" />
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SwitchField
            control={control}
            name="isAutomatic"
            label="Automatic"
            description="Apply without a code when the cart qualifies."
          />
          <SwitchField
            control={control}
            name="isTaxInclusive"
            label="Tax inclusive"
            description="The value already includes tax."
          />
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField control={control} name="startsAt" label="Starts at" type="date" optional />
          <TextField control={control} name="endsAt" label="Ends at" type="date" optional />
        </div>

        <TextField
          control={control}
          name="campaignId"
          label="Campaign ID"
          optional
          hint="Attach this promotion to an existing campaign."
          placeholder="camp_…"
        />
      </div>

      {extra}
    </ResourceFormLayout>
  )
}
