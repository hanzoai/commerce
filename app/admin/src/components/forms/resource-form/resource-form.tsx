'use client'

// The ONE form engine: react-hook-form + zod validation driven by a `fields` data
// table. Customer + customer-group, create + edit, all compose this — no copy-paste
// form markup anywhere. Give it a schema, defaults, and a field list; it renders
// labeled+validated inputs and a Cancel/Submit footer, and calls `onSubmit(values)`
// only when valid. `isPending` drives the submit button's spinner.
import { zodResolver } from '@hookform/resolvers/zod'
import { Button } from '@hanzo/commerce-ui'
import { useForm, type DefaultValues, type FieldValues } from 'react-hook-form'
import type { z, ZodType } from 'zod'
import { Form } from '@/components/common/form'
import { FieldRow, type FieldSpec } from './field-row'

interface ResourceFormProps<TSchema extends ZodType<FieldValues>> {
  schema: TSchema
  defaultValues: DefaultValues<z.infer<TSchema>>
  fields: FieldSpec<z.infer<TSchema>>[]
  onSubmit: (values: z.infer<TSchema>) => void | Promise<void>
  onCancel?: () => void
  submitLabel?: string
  cancelLabel?: string
  isPending?: boolean
  /** One column instead of the default two-up responsive grid. */
  single?: boolean
}

export function ResourceForm<TSchema extends ZodType<FieldValues>>({
  schema,
  defaultValues,
  fields,
  onSubmit,
  onCancel,
  submitLabel = 'Save',
  cancelLabel = 'Cancel',
  isPending,
  single,
}: ResourceFormProps<TSchema>) {
  type Values = z.infer<TSchema>
  const form = useForm<Values>({
    defaultValues,
    resolver: zodResolver(schema),
    mode: 'onSubmit',
  })

  const submit = form.handleSubmit(async (values) => {
    await onSubmit(values)
  })

  return (
    <Form {...form}>
      <form onSubmit={submit} className="flex flex-col gap-y-6">
        <div className={single ? 'flex flex-col gap-4' : 'grid grid-cols-1 gap-4 md:grid-cols-2'}>
          {fields.map((spec) => (
            <FieldRow key={String(spec.name)} control={form.control} spec={spec} />
          ))}
        </div>
        <div className="flex items-center justify-end gap-x-2">
          {onCancel && (
            <Button type="button" size="small" variant="secondary" onClick={onCancel} disabled={isPending}>
              {cancelLabel}
            </Button>
          )}
          <Button type="submit" size="small" variant="primary" isLoading={isPending}>
            {submitLabel}
          </Button>
        </div>
      </form>
    </Form>
  )
}
