'use client'

import { useId } from 'react'
import { Controller, type Control, type FieldPath, type FieldValues } from 'react-hook-form'
import { Select } from '@hanzo/commerce-ui'
import { FieldRow } from './form-fields'

export interface SelectOption {
  value: string
  label: string
}

interface SelectFieldProps<T extends FieldValues> {
  control: Control<T>
  name: FieldPath<T>
  label: string
  options: SelectOption[]
  placeholder?: string
  optional?: boolean
  disabled?: boolean
  className?: string
}

/** A labelled, validated Select bound to a react-hook-form control — the select
 *  counterpart to form-fields' TextField, sharing the same one <FieldRow> layout. */
export function SelectField<T extends FieldValues>({
  control,
  name,
  label,
  options,
  placeholder,
  optional,
  disabled,
  className,
}: SelectFieldProps<T>) {
  const id = useId()
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldRow id={id} label={label} optional={optional} error={fieldState.error?.message} className={className}>
          <Select value={field.value ?? ''} onValueChange={field.onChange} disabled={disabled}>
            <Select.Trigger ref={field.ref}>
              <Select.Value placeholder={placeholder} />
            </Select.Trigger>
            <Select.Content>
              {options.map((option) => (
                <Select.Item key={option.value} value={option.value}>
                  {option.label}
                </Select.Item>
              ))}
            </Select.Content>
          </Select>
        </FieldRow>
      )}
    />
  )
}
