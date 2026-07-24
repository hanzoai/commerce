'use client'

import { memo } from 'react'
import { useFieldArray, type Control, type UseFormRegister } from 'react-hook-form'
import { Button, Input, Text, IconButton } from '@hanzo/commerce-ui'
import { Plus, Trash } from '@hanzo/commerce-icons'
import { Field } from '@/components/common/field'
import type { ProductFormValues } from '@/lib/products/product'

interface OptionsEditorProps {
  control: Control<ProductFormValues>
  register: UseFormRegister<ProductFormValues>
  disabled?: boolean
}

/** Inline editor for a product's options (e.g. Size → S, M, L). */
function OptionsEditorImpl({ control, register, disabled }: OptionsEditorProps) {
  const { fields, append, remove } = useFieldArray({ control, name: 'options' })

  return (
    <div className="flex flex-col gap-y-4">
      {fields.length === 0 && (
        <Text size="small" className="text-ui-fg-muted">
          No options yet. Add one for variant axes like size or color.
        </Text>
      )}
      {fields.map((field, index) => (
        <div key={field.id} className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_2fr_auto] sm:items-end">
          <Field label="Name">
            <Input placeholder="Size" disabled={disabled} {...register(`options.${index}.name`)} />
          </Field>
          <Field label="Values" hint="Comma-separated">
            <Input placeholder="S, M, L" disabled={disabled} {...register(`options.${index}.values`)} />
          </Field>
          <IconButton
            type="button"
            variant="transparent"
            disabled={disabled}
            onClick={() => remove(index)}
            aria-label="Remove option"
          >
            <Trash />
          </IconButton>
        </div>
      ))}
      {!disabled && (
        <div>
          <Button type="button" variant="secondary" size="small" onClick={() => append({ name: '', values: '' })}>
            <Plus /> Add option
          </Button>
        </div>
      )}
    </div>
  )
}

export const OptionsEditor = memo(OptionsEditorImpl)
