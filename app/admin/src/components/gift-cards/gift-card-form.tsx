'use client'

// The gift-card create/edit form. One <form> composed from the shared Field /
// Fieldset primitives and driven by react-hook-form + the giftCardSchema, so the
// create page and the edit detail render the exact same fields. Code, currency
// and initial balance are immutable after issue (the balance is a ledger
// projection), so in edit mode they render read-only.

import { useRouter } from 'next/navigation'
import { useForm, Controller, type Control } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Input, Select, Switch, Text, Textarea, toast } from '@hanzo/commerce-ui'
import { Field, Fieldset } from '@/components/common/field'
import { ConfirmButton } from '@/components/common/confirm-button'
import { useCreate, useUpdate, useDelete } from '@/lib/api/hooks'
import { formatMoney } from '@/lib/format'
import {
  giftCardSchema,
  emptyForm,
  giftCardToForm,
  formToCreatePayload,
  formToEditPayload,
  CURRENCIES,
  type GiftCard,
  type GiftCardFormValues,
} from '@/lib/gift-cards/gift-card'

interface GiftCardFormProps {
  mode: 'create' | 'edit'
  card?: GiftCard
}

export function GiftCardForm({ mode, card }: GiftCardFormProps) {
  const router = useRouter()

  const create = useCreate<GiftCard>('gift-card')
  const update = useUpdate<GiftCard>('gift-card')
  const del = useDelete('gift-card')

  const {
    register,
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<GiftCardFormValues>({
    resolver: zodResolver(giftCardSchema),
    defaultValues: mode === 'edit' && card ? giftCardToForm(card) : emptyForm(),
  })

  const onSubmit = handleSubmit(async (values) => {
    try {
      if (mode === 'create') {
        await create.mutateAsync(formToCreatePayload(values))
        toast.success('Gift card created')
      } else {
        await update.mutateAsync({ id: card!.id, data: formToEditPayload(values) })
        toast.success('Gift card updated')
      }
      router.push('/gift-cards')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not save the gift card')
    }
  })

  const onDelete = async () => {
    await del.mutateAsync(card!.id)
    toast.success('Gift card deleted')
    router.push('/gift-cards')
  }

  const busy = create.isPending || update.isPending

  return (
    <form onSubmit={onSubmit} className="flex w-full flex-col gap-y-6">
      <Fieldset title="Details" description="Face value and currency are fixed once the card is issued.">
        {mode === 'create' ? (
          <>
            <Field label="Code" error={errors.code?.message} hint="The code a customer redeems. Stored uppercase.">
              <Input autoFocus placeholder="GIFT-4F2A-9C1D" {...register('code')} />
            </Field>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Currency" error={errors.currency?.message}>
                <Controller
                  control={control}
                  name="currency"
                  render={({ field: { onChange, value } }) => (
                    <Select value={value} onValueChange={onChange}>
                      <Select.Trigger>
                        <Select.Value placeholder="Currency" />
                      </Select.Trigger>
                      <Select.Content>
                        {CURRENCIES.map((c) => (
                          <Select.Item key={c} value={c}>
                            {c.toUpperCase()}
                          </Select.Item>
                        ))}
                      </Select.Content>
                    </Select>
                  )}
                />
              </Field>
              <Field label="Initial balance" error={errors.initialBalance?.message} hint="Face value in the selected currency.">
                <Input inputMode="decimal" placeholder="50.00" {...register('initialBalance')} />
              </Field>
            </div>
          </>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <ReadOnly label="Code" value={card?.code ?? '—'} />
            <ReadOnly label="Currency" value={(card?.currency ?? 'usd').toUpperCase()} />
            <ReadOnly label="Initial value" value={formatMoney(card?.initialBalanceCents, card?.currency)} />
          </div>
        )}
      </Fieldset>

      <Fieldset title="Scope & lifecycle" description="Where the card is valid and when it stops working.">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Region ID" optional hint="Restrict to one region; blank = any.">
            <Input placeholder="reg_…" {...register('regionId')} />
          </Field>
          <Field label="Order ID" optional hint="The order this card was purchased on.">
            <Input placeholder="order_…" {...register('orderId')} />
          </Field>
        </div>
        <Field label="Expires at" optional error={errors.endsAt?.message} hint="After this the card can't be redeemed.">
          <Input type="datetime-local" {...register('endsAt')} />
        </Field>
        <DisabledRow control={control} />
      </Fieldset>

      <Fieldset title="Metadata" description="Optional JSON object stored with the card.">
        <Field label="Metadata" optional error={errors.metadata?.message}>
          <Textarea rows={4} placeholder={'{\n  "note": "holiday promo"\n}'} {...register('metadata')} />
        </Field>
      </Fieldset>

      <div className="flex items-center justify-between gap-2 border-t border-ui-border-base pt-4">
        <div>
          {mode === 'edit' && (
            <ConfirmButton
              onConfirm={onDelete}
              title="Delete gift card"
              description="This permanently removes the card. Its redemption history is kept for audit."
            >
              Delete
            </ConfirmButton>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button type="button" variant="secondary" size="small" onClick={() => router.push('/gift-cards')} disabled={busy}>
            Cancel
          </Button>
          <Button type="submit" size="small" isLoading={busy}>
            {mode === 'create' ? 'Create gift card' : 'Save changes'}
          </Button>
        </div>
      </div>
    </form>
  )
}

/** A read-only detail value, laid out like a Field but without a control. */
function ReadOnly({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-y-1.5">
      <Text size="small" weight="plus" className="text-ui-fg-base">
        {label}
      </Text>
      <Text size="small" className="text-ui-fg-subtle">
        {value}
      </Text>
    </div>
  )
}

/** The one labeled switch row — a boolean bound to the form control. */
function DisabledRow({ control }: { control: Control<GiftCardFormValues> }) {
  return (
    <div className="flex items-start gap-x-3 rounded-lg border border-ui-border-base bg-ui-bg-base p-3">
      <Controller
        control={control}
        name="disabled"
        render={({ field: { value, onChange } }) => <Switch checked={value} onCheckedChange={onChange} />}
      />
      <div className="min-w-0">
        <Text size="small" weight="plus" className="text-ui-fg-base">
          Disabled
        </Text>
        <Text size="small" leading="compact" className="text-ui-fg-subtle">
          Disabled cards can&apos;t be redeemed even if a balance remains.
        </Text>
      </div>
    </div>
  )
}
