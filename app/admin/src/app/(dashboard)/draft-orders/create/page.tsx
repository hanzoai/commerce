'use client'

// Create a draft order shell (customer + currency), then land on its detail
// page where the line-item builder composes the order and completes it. This is
// the real "create an order for a customer" flow — the record-only orders/create
// form has no line-item builder.

import { useRouter } from 'next/navigation'
import { useForm, Controller } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Container, Input, Select, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { Field, Fieldset } from '@/components/common/field'
import { ToasterMount } from '@/components/common/toaster-mount'
import { useCreate } from '@/lib/api/hooks'
import {
  draftOrderSchema,
  emptyForm,
  formToCreatePayload,
  CURRENCIES,
  type DraftOrder,
  type DraftOrderFormValues,
} from '@/lib/draft-orders/draft-order'

export default function CreateDraftOrderPage() {
  const router = useRouter()
  const create = useCreate<DraftOrder>('draft-order')

  const {
    register,
    control,
    handleSubmit,
    formState: { errors },
  } = useForm<DraftOrderFormValues>({
    resolver: zodResolver(draftOrderSchema),
    defaultValues: emptyForm(),
  })

  const onSubmit = handleSubmit(async (values) => {
    try {
      const draft = await create.mutateAsync(formToCreatePayload(values))
      toast.success('Draft order created')
      router.push(`/draft-orders/${draft.id}`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not create the draft order')
    }
  })

  return (
    <div>
      <ToasterMount />
      <PageHeader title="New draft order" description="Start an order for a customer; add line items next." />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          <form onSubmit={onSubmit} className="flex flex-col gap-y-6">
            <Fieldset title="Customer" description="Who this order is being built for.">
              <Field label="Email" optional error={errors.email?.message} hint="Where the order confirmation is sent.">
                <Input autoFocus type="email" placeholder="buyer@example.com" {...register('email')} />
              </Field>
              <Field label="Customer ID" optional hint="Link this draft to an existing customer.">
                <Input placeholder="user_…" {...register('customerId')} />
              </Field>
            </Fieldset>

            <Fieldset title="Currency" description="The currency every line item is priced in.">
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
            </Fieldset>

            <div className="flex items-center justify-end gap-2">
              <Button
                type="button"
                size="small"
                variant="secondary"
                onClick={() => router.push('/draft-orders')}
                disabled={create.isPending}
              >
                Cancel
              </Button>
              <Button type="submit" size="small" isLoading={create.isPending}>
                Create & add items
              </Button>
            </div>
          </form>
        </Container>
      </div>
    </div>
  )
}
