'use client'

import { forwardRef } from 'react'
import { Input, Label, Text, clx } from '@hanzo/commerce-ui'

interface FieldRowProps extends React.ComponentPropsWithoutRef<typeof Input> {
  label: string
  hint?: string
  error?: string
}

/**
 * One labeled input — the single field primitive the paywall (and any ported
 * resource form) composes from. `ref` forwards to the underlying Input so
 * react-hook-form's `register` wires straight through.
 */
export const FieldRow = forwardRef<HTMLInputElement, FieldRowProps>(function FieldRow(
  { label, hint, error, id, className, ...props },
  ref,
) {
  const fieldId = id ?? `field-${label.toLowerCase().replace(/\s+/g, '-')}`
  return (
    <div className={clx('flex flex-col gap-y-1.5', className)}>
      <Label htmlFor={fieldId} size="small" weight="plus">
        {label}
      </Label>
      <Input
        id={fieldId}
        ref={ref}
        aria-invalid={!!error}
        className={clx(error && 'border-ui-border-error')}
        {...props}
      />
      {error ? (
        <Text size="xsmall" className="text-ui-fg-error">{error}</Text>
      ) : hint ? (
        <Text size="xsmall" className="text-ui-fg-muted">{hint}</Text>
      ) : null}
    </div>
  )
})
