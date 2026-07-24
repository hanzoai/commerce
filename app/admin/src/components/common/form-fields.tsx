'use client'

// Reusable, self-contained form-field primitives composed from @hanzo/commerce-ui
// and wired to react-hook-form via <Controller>. One <FieldRow> owns the
// label + control + error/hint layout; the typed field wrappers bind a control.
// The live admin surface does NOT mount react-i18next, so these use plain strings
// (matching create-product.tsx) rather than the Medusa `common/form` primitive.

import { useId, type ReactNode } from 'react'
import { Controller, type Control, type FieldPath, type FieldValues } from 'react-hook-form'
import { Input, Label, Text, Textarea, clx } from '@hanzo/commerce-ui'

interface FieldRowProps {
  id?: string
  label: string
  optional?: boolean
  error?: string
  hint?: string
  className?: string
  children: ReactNode
}

/** Label + control + error/hint stack — the one field layout every input reuses. */
export function FieldRow({ id, label, optional, error, hint, className, children }: FieldRowProps) {
  return (
    <div className={clx('flex flex-col gap-y-2', className)}>
      <div className="flex items-center gap-x-1">
        <Label htmlFor={id} size="small" weight="plus">
          {label}
        </Label>
        {optional && (
          <Text size="small" leading="compact" className="text-ui-fg-muted">
            (Optional)
          </Text>
        )}
      </div>
      {children}
      {hint && !error && (
        <Text size="small" leading="compact" className="text-ui-fg-subtle">
          {hint}
        </Text>
      )}
      {error && (
        <Text size="small" leading="compact" className="text-ui-fg-error">
          {error}
        </Text>
      )}
    </div>
  )
}

interface FieldProps<T extends FieldValues> {
  control: Control<T>
  name: FieldPath<T>
  label: string
  optional?: boolean
  hint?: string
  placeholder?: string
  disabled?: boolean
  autoFocus?: boolean
  className?: string
}

/** A labelled, validated text input bound to a react-hook-form control. */
export function TextField<T extends FieldValues>({
  control,
  name,
  label,
  optional,
  hint,
  placeholder,
  disabled,
  autoFocus,
  className,
  type = 'text',
}: FieldProps<T> & { type?: string }) {
  const id = useId()
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldRow
          id={id}
          label={label}
          optional={optional}
          hint={hint}
          error={fieldState.error?.message}
          className={className}
        >
          <Input
            id={id}
            {...field}
            value={field.value ?? ''}
            type={type}
            placeholder={placeholder}
            disabled={disabled}
            autoFocus={autoFocus}
          />
        </FieldRow>
      )}
    />
  )
}

/** A labelled, validated multi-line input bound to a react-hook-form control. */
export function TextareaField<T extends FieldValues>({
  control,
  name,
  label,
  optional,
  hint,
  placeholder,
  disabled,
  className,
  rows = 4,
}: FieldProps<T> & { rows?: number }) {
  const id = useId()
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <FieldRow
          id={id}
          label={label}
          optional={optional}
          hint={hint}
          error={fieldState.error?.message}
          className={className}
        >
          <Textarea
            id={id}
            {...field}
            value={field.value ?? ''}
            rows={rows}
            placeholder={placeholder}
            disabled={disabled}
          />
        </FieldRow>
      )}
    />
  )
}
