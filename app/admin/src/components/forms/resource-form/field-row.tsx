'use client'

// ONE labeled field. Every form field in the admin renders through this, so the
// label / control / error markup lives in exactly one place. Driven by a plain
// `FieldSpec` data record (no per-field JSX), which is what makes the forms DRY.
import { Input, Switch, Textarea } from '@hanzo/commerce-ui'
import type { Control, FieldPath, FieldValues } from 'react-hook-form'
import { Form } from '@/components/common/form'

export type FieldKind = 'text' | 'email' | 'password' | 'tel' | 'textarea' | 'switch'

export interface FieldSpec<T extends FieldValues> {
  name: FieldPath<T>
  label: string
  kind?: FieldKind
  placeholder?: string
  optional?: boolean
  disabled?: boolean
  autoComplete?: string
}

interface FieldRowProps<T extends FieldValues> {
  control: Control<T>
  spec: FieldSpec<T>
}

export function FieldRow<T extends FieldValues>({ control, spec }: FieldRowProps<T>) {
  const { name, label, kind = 'text', placeholder, optional, disabled, autoComplete } = spec

  return (
    <Form.Field
      control={control}
      name={name}
      render={({ field }) => (
        <Form.Item>
          <Form.Label optional={optional}>{label}</Form.Label>
          <Form.Control>
            {kind === 'switch' ? (
              <Switch
                checked={!!field.value}
                onCheckedChange={field.onChange}
                onBlur={field.onBlur}
                disabled={disabled}
              />
            ) : kind === 'textarea' ? (
              <Textarea
                {...field}
                value={field.value ?? ''}
                placeholder={placeholder}
                disabled={disabled}
                autoComplete={autoComplete}
              />
            ) : (
              <Input
                {...field}
                value={field.value ?? ''}
                type={kind === 'tel' ? 'tel' : kind}
                placeholder={placeholder}
                disabled={disabled}
                autoComplete={autoComplete ?? 'off'}
              />
            )}
          </Form.Control>
          <Form.ErrorMessage />
        </Form.Item>
      )}
    />
  )
}
