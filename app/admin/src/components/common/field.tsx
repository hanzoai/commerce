'use client'

import { Label, Text, clx } from '@hanzo/commerce-ui'
import { useId } from 'react'

interface FieldProps {
  label: string
  htmlFor?: string
  optional?: boolean
  hint?: string
  error?: string
  className?: string
  children: React.ReactNode
}

/**
 * The one labeled-field wrapper used by every form control in the admin:
 * label (+ optional tag), the control, an optional hint, and an error line.
 * Self-contained — no form context, no i18n — so it composes with plain inputs,
 * Controllers, or anything else.
 */
export function Field({ label, htmlFor, optional, hint, error, className, children }: FieldProps) {
  const autoId = useId()
  const id = htmlFor ?? autoId
  return (
    <div className={clx('flex flex-col gap-y-1.5', className)}>
      <div className="flex items-center gap-x-1">
        <Label htmlFor={id} size="small" weight="plus">
          {label}
        </Label>
        {optional && (
          <Text size="small" leading="compact" className="text-ui-fg-muted">
            (optional)
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

interface FieldsetProps {
  title: string
  description?: string
  actions?: React.ReactNode
  className?: string
  children: React.ReactNode
}

/** A titled card that groups related fields into a section of a form. */
export function Fieldset({ title, description, actions, className, children }: FieldsetProps) {
  return (
    <section
      className={clx(
        'rounded-lg border border-ui-border-base bg-ui-bg-subtle shadow-elevation-card-rest',
        className,
      )}
    >
      <header className="flex items-start justify-between gap-x-4 border-b border-ui-border-base px-5 py-4">
        <div>
          <Text weight="plus" size="small" className="text-ui-fg-base">
            {title}
          </Text>
          {description && (
            <Text size="small" leading="compact" className="mt-0.5 text-ui-fg-subtle">
              {description}
            </Text>
          )}
        </div>
        {actions}
      </header>
      <div className="flex flex-col gap-y-4 p-5">{children}</div>
    </section>
  )
}
