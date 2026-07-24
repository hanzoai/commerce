'use client'

import { useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Badge, Button, Input, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { SelectField, SwitchField } from '@/components/common/choice-fields'
import { ConfirmButton } from '@/components/common/confirm-button'
import {
  ALLOCATION_OPTIONS,
  CURRENCY_OPTIONS,
  STATUS_OPTIONS,
  TARGET_TYPE_OPTIONS,
  TYPE_OPTIONS,
  VALUE_TYPE_OPTIONS,
  currencySymbol,
  discountSchema,
  emptyDiscount,
  promotionToForm,
  statusColor,
  useDeleteDiscount,
  useSaveDiscount,
  type DiscountFormValues,
  type Promotion,
} from '@/lib/discounts'

interface DiscountFormProps {
  mode: 'create' | 'edit'
  promotion?: Promotion
}

export function DiscountForm({ mode, promotion }: DiscountFormProps) {
  const router = useRouter()
  const { save, isPending: saving } = useSaveDiscount()
  const { remove, isPending: deleting } = useDeleteDiscount()

  const {
    register,
    control,
    handleSubmit,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<DiscountFormValues>({
    resolver: zodResolver(discountSchema),
    defaultValues: mode === 'edit' && promotion ? promotionToForm(promotion) : emptyDiscount(),
  })

  const valueType = watch('valueType')
  const type = watch('type')
  const allocation = watch('allocation')
  const status = watch('status')
  const currencyCode = watch('currencyCode')
  const isFixed = valueType === 'fixed'
  const isStandard = type === 'standard'

  const onSubmit = handleSubmit(async (values) => {
    try {
      await save(values, mode === 'edit' ? promotion : undefined)
      toast.success(mode === 'create' ? 'Discount created' : 'Discount updated')
      router.push('/discounts')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not save the discount')
    }
  })

  const onDelete = async () => {
    if (!promotion) return
    await remove(promotion)
    toast.success('Discount deleted')
    router.push('/discounts')
  }

  const busy = isSubmitting || saving

  return (
    <form onSubmit={onSubmit} className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
      <Fieldset
        title="Details"
        description="How shoppers find and redeem this discount."
        actions={
          <Badge size="2xsmall" color={statusColor(status)}>
            {status}
          </Badge>
        }
      >
        <Field label="Code" error={errors.code?.message} hint="The code a shopper enters at checkout.">
          <Input autoFocus={mode === 'create'} placeholder="SUMMER15" {...register('code')} />
        </Field>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField control={control} name="status" label="Status" options={STATUS_OPTIONS} placeholder="Select status" />
          <SelectField control={control} name="type" label="Type" options={TYPE_OPTIONS} placeholder="Select type" />
        </div>
      </Fieldset>

      <Fieldset title="Value" description="The discount applied when this promotion matches.">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField
            control={control}
            name="valueType"
            label="Value type"
            options={VALUE_TYPE_OPTIONS}
            placeholder="Select value type"
          />
          <Field label={isFixed ? 'Amount' : 'Percentage'} error={errors.value?.message}>
            <div className="relative">
              <span className="pointer-events-none absolute inset-y-0 left-2.5 flex items-center text-ui-fg-muted">
                {isFixed ? currencySymbol(currencyCode) : '%'}
              </span>
              <Input
                inputMode="decimal"
                placeholder={isFixed ? '5.00' : '15'}
                className="pl-6"
                {...register('value')}
              />
            </div>
          </Field>
        </div>
        {isFixed && (
          <SelectField
            control={control}
            name="currencyCode"
            label="Currency"
            options={CURRENCY_OPTIONS}
            placeholder="Select currency"
            className="sm:max-w-xs"
          />
        )}
      </Fieldset>

      {isStandard && (
        <Fieldset title="Application" description="Which items the discount applies to, and how.">
          <SelectField
            control={control}
            name="targetType"
            label="Applies to"
            options={TARGET_TYPE_OPTIONS}
            placeholder="Select target"
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectField
              control={control}
              name="allocation"
              label="Allocation"
              options={ALLOCATION_OPTIONS}
              placeholder="Select allocation"
            />
            {allocation !== 'across' && (
              <Field
                label="Max quantity"
                optional
                error={errors.maxQuantity?.message}
                hint="Cap the number of items discounted."
              >
                <Input inputMode="numeric" placeholder="3" {...register('maxQuantity')} />
              </Field>
            )}
          </div>
        </Fieldset>
      )}

      <Fieldset title="Method" description="How the discount is triggered and taxed.">
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
          description="The discount value already includes tax."
        />
      </Fieldset>

      <Fieldset title="Schedule" description="Optionally limit when the discount is active.">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Starts at" optional error={errors.startsAt?.message}>
            <Input type="date" {...register('startsAt')} />
          </Field>
          <Field label="Ends at" optional error={errors.endsAt?.message}>
            <Input type="date" {...register('endsAt')} />
          </Field>
        </div>
        <Field label="Campaign ID" optional hint="Attach this discount to an existing campaign.">
          <Input placeholder="camp_…" {...register('campaignId')} />
        </Field>
      </Fieldset>

      <footer className="sticky bottom-0 z-10 -mx-4 flex items-center gap-x-2 border-t border-ui-border-base bg-ui-bg-base px-4 py-3 sm:-mx-8 sm:px-8">
        {mode === 'edit' && (
          <ConfirmButton
            onConfirm={onDelete}
            title="Delete discount"
            description="This permanently removes the discount. This cannot be undone."
            disabled={deleting}
          >
            Delete
          </ConfirmButton>
        )}
        <div className="ml-auto flex items-center gap-x-2">
          <Button type="button" variant="secondary" size="small" onClick={() => router.push('/discounts')}>
            Cancel
          </Button>
          <Button type="submit" size="small" isLoading={busy}>
            {mode === 'create' ? 'Create discount' : 'Save changes'}
          </Button>
        </div>
      </footer>
    </form>
  )
}
