'use client'

import { memo } from 'react'
import { useFieldArray, Controller, type Control, type UseFormRegister } from 'react-hook-form'
import { Button, Input, Switch, Text, IconButton } from '@hanzo/commerce-ui'
import { Plus, Trash } from '@hanzo/commerce-icons'
import { Field } from '@/components/common/field'
import { symbolFor, type ProductFormValues } from '@/lib/products/product'

interface VariantsEditorProps {
  control: Control<ProductFormValues>
  register: UseFormRegister<ProductFormValues>
  currency: string
  disabled?: boolean
}

/** Editor for a product's sellable variants (name, SKU, price, availability). */
function VariantsEditorImpl({ control, register, currency, disabled }: VariantsEditorProps) {
  const { fields, append, remove } = useFieldArray({ control, name: 'variants' })
  const symbol = symbolFor(currency)

  return (
    <div className="flex flex-col gap-y-4">
      {fields.length === 0 && (
        <Text size="small" className="text-ui-fg-muted">
          No variants yet. Add one per purchasable SKU.
        </Text>
      )}
      {fields.map((field, index) => (
        <div
          key={field.id}
          className="grid grid-cols-1 gap-3 rounded-lg border border-ui-border-base bg-ui-bg-base p-3 sm:grid-cols-[2fr_1.5fr_1fr_auto_auto] sm:items-end"
        >
          <Field label="Name">
            <Input placeholder="Small / Black" disabled={disabled} {...register(`variants.${index}.name`)} />
          </Field>
          <Field label="SKU">
            <Input placeholder="TEE-BLK-S" disabled={disabled} {...register(`variants.${index}.sku`)} />
          </Field>
          <Field label={`Price (${currency})`}>
            <div className="relative">
              <span className="pointer-events-none absolute inset-y-0 left-2.5 flex items-center text-ui-fg-muted">
                {symbol}
              </span>
              <Input
                inputMode="decimal"
                placeholder="0.00"
                className="pl-6"
                disabled={disabled}
                {...register(`variants.${index}.price`)}
              />
            </div>
          </Field>
          <Field label="Available">
            <Controller
              control={control}
              name={`variants.${index}.available`}
              render={({ field: f }) => (
                <Switch checked={f.value} onCheckedChange={f.onChange} disabled={disabled} />
              )}
            />
          </Field>
          <IconButton
            type="button"
            variant="transparent"
            disabled={disabled}
            onClick={() => remove(index)}
            aria-label="Remove variant"
          >
            <Trash />
          </IconButton>
        </div>
      ))}
      {!disabled && (
        <div>
          <Button
            type="button"
            variant="secondary"
            size="small"
            onClick={() => append({ name: '', sku: '', price: '', available: true })}
          >
            <Plus /> Add variant
          </Button>
        </div>
      )}
    </div>
  )
}

export const VariantsEditor = memo(VariantsEditorImpl)
