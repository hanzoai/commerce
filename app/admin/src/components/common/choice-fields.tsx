'use client'

// Controlled Select + Switch field primitives — the choice counterparts to the
// text/textarea wrappers, bound to a react-hook-form control by name. One place
// for the "labeled select" and "labeled switch" layouts every resource form reuses,
// so no form re-wires Controller + label/error markup per option input.

import { Controller, type Control, type FieldPath, type FieldValues } from 'react-hook-form'
import { Select, Switch, Text, clx } from '@hanzo/commerce-ui'
import { Field } from './field'

interface Option {
  value: string
  label: string
}

export function SelectField<T extends FieldValues>({
  control,
  name,
  label,
  hint,
  options,
  placeholder,
  disabled,
  className,
}: {
  control: Control<T>
  name: FieldPath<T>
  label: string
  hint?: string
  options: readonly Option[]
  placeholder?: string
  disabled?: boolean
  className?: string
}) {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <Field label={label} hint={hint} error={fieldState.error?.message} className={className}>
          <Select
            value={(field.value as string | undefined) ?? ''}
            onValueChange={field.onChange}
            disabled={disabled}
          >
            <Select.Trigger ref={field.ref}>
              <Select.Value placeholder={placeholder} />
            </Select.Trigger>
            <Select.Content>
              {options.map((o) => (
                <Select.Item key={o.value} value={o.value}>
                  {o.label}
                </Select.Item>
              ))}
            </Select.Content>
          </Select>
        </Field>
      )}
    />
  )
}

export function SwitchField<T extends FieldValues>({
  control,
  name,
  label,
  description,
  className,
}: {
  control: Control<T>
  name: FieldPath<T>
  label: string
  description?: string
  className?: string
}) {
  return (
    <div
      className={clx(
        'flex items-start gap-x-3 rounded-lg border border-ui-border-base bg-ui-bg-base p-3',
        className,
      )}
    >
      <Controller
        control={control}
        name={name}
        render={({ field: { value, onChange } }) => (
          <Switch checked={!!value} onCheckedChange={onChange} />
        )}
      />
      <div className="min-w-0">
        <Text size="small" weight="plus" className="text-ui-fg-base">
          {label}
        </Text>
        {description && (
          <Text size="small" leading="compact" className="text-ui-fg-subtle">
            {description}
          </Text>
        )}
      </div>
    </div>
  )
}
