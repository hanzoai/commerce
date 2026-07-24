'use client'

// Detail + edit view for one v2 promotion. Reads its id from the route params,
// fetches the record client-side, and renders the shared <PromotionForm>
// pre-filled — plus two panels injected below the fields: the application method
// (the discount value the promotion carries) and a live evaluate preview
// (POST /v1/promotion/evaluate). Save updates, Delete removes with a confirm.

import { useMemo, useRef, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Container, Input, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useGet, useUpdate, useDelete, useList, useCreate } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'
import { DetailPanel } from '@/components/common/detail-panel'
import { TextField } from '@/components/common/form-fields'
import { SelectField } from '@/components/common/choice-fields'
import { PromotionForm } from './promotion-form'
import {
  toPayload,
  toValues,
  methodSchema,
  methodToPayload,
  methodToValues,
  emptyMethod,
  METHOD_TYPE_OPTIONS,
  TARGET_TYPE_OPTIONS,
  ALLOCATION_OPTIONS,
  CURRENCY_OPTIONS,
  type Promotion,
  type PromotionValues,
  type ApplicationMethod,
  type MethodValues,
  type EvaluateResponse,
} from '@/lib/promotions/promotion'

function paramId(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Promotion" description="Loading…" />
      <div className="p-8">
        <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-md" />
          ))}
        </Container>
      </div>
    </div>
  )
}

export function PromotionDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<Promotion>('promotion', id)
  const update = useUpdate<Promotion>('promotion')
  const remove = useDelete('promotion')

  if (isLoading || !id) return <LoadingState />

  if (!data) {
    return (
      <div>
        <PageHeader title="Promotion" description="This promotion could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/promotions')}
          >
            Back to promotions
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: PromotionValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Promotion updated')
      router.push('/promotions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the promotion')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Promotion deleted')
      router.push('/promotions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the promotion')
    }
  }

  return (
    <PromotionForm
      title={data.code || 'Promotion'}
      description="Edit this promotion, its application method, and preview how it applies."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      extra={
        <div className="flex flex-col gap-y-6">
          <MethodPanel promotionId={id} />
          <EvaluatePanel />
        </div>
      }
    />
  )
}

// ── Application method panel ──────────────────────────────────────────────────

function MethodPanel({ promotionId }: { promotionId: string }) {
  const { data, isLoading } = useList<ApplicationMethod>('applicationmethod', { display: 200 })
  const create = useCreate<ApplicationMethod>('applicationmethod')
  const update = useUpdate<ApplicationMethod>('applicationmethod')

  const existing = useMemo(
    () => (data?.models ?? []).find((m) => m.promotionId === promotionId),
    [data, promotionId],
  )

  const { control, handleSubmit, watch, reset } = useForm<MethodValues>({
    defaultValues: existing ? methodToValues(existing) : emptyMethod,
    resolver: zodResolver(methodSchema),
  })
  // Re-seed once the method loads (the list resolves after first render).
  const seededId = useRef(existing?.id)
  if (existing && seededId.current !== existing.id) {
    seededId.current = existing.id
    reset(methodToValues(existing))
  }

  const isFixed = watch('type') === 'fixed'

  const save = async (values: MethodValues) => {
    const payload = methodToPayload(values, promotionId)
    try {
      if (existing) await update.mutateAsync({ id: existing.id, data: payload })
      else await create.mutateAsync(payload)
      toast.success('Application method saved')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not save the application method')
    }
  }

  const busy = create.isPending || update.isPending

  return (
    <DetailPanel
      title="Application method"
      description="The discount this promotion applies when it matches a cart."
    >
      {isLoading ? (
        <Skeleton className="h-24 w-full rounded-md" />
      ) : (
        <div className="flex flex-col gap-y-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectField control={control} name="type" label="Type" options={METHOD_TYPE_OPTIONS} placeholder="Select type" />
            <TextField
              control={control}
              name="value"
              label={isFixed ? 'Amount (cents)' : 'Percentage (basis points)'}
              placeholder={isFixed ? '500' : '1500'}
              hint={isFixed ? '500 = $5.00' : '1500 = 15.00%'}
            />
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectField control={control} name="targetType" label="Applies to" options={TARGET_TYPE_OPTIONS} placeholder="Select target" />
            <SelectField control={control} name="allocation" label="Allocation" options={ALLOCATION_OPTIONS} placeholder="Select allocation" />
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
          <div className="flex justify-end">
            <Button type="button" size="small" isLoading={busy} onClick={handleSubmit(save)}>
              {existing ? 'Save method' : 'Add method'}
            </Button>
          </div>
        </div>
      )}
    </DetailPanel>
  )
}

// ── Evaluate preview panel ────────────────────────────────────────────────────

function EvaluatePanel() {
  const evaluate = useCreate<EvaluateResponse>('promotion/evaluate')
  const [currency, setCurrency] = useState('usd')
  const [cartTotal, setCartTotal] = useState('10000')
  const [result, setResult] = useState<EvaluateResponse | null>(null)

  const run = async () => {
    try {
      const res = await evaluate.mutateAsync({
        currencyCode: currency,
        cartTotal: Number(cartTotal) || 0,
        items: [],
      } as any)
      setResult(res)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Evaluate failed')
    }
  }

  return (
    <DetailPanel
      title="Preview"
      description="Evaluate the active automatic promotions against a sample cart."
      action={
        <Button type="button" size="small" variant="secondary" isLoading={evaluate.isPending} onClick={run}>
          Evaluate
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Currency</Text>
          <Input value={currency} onChange={(e) => setCurrency(e.target.value)} placeholder="usd" />
        </label>
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Cart total (cents)</Text>
          <Input value={cartTotal} onChange={(e) => setCartTotal(e.target.value)} inputMode="numeric" placeholder="10000" />
        </label>
      </div>
      {result && (
        <div className="rounded-lg border border-ui-border-base p-4">
          <Text size="small" weight="plus" className="text-ui-fg-base">
            Total discount: {result.totalDiscount} cents
          </Text>
          {result.adjustments.length === 0 ? (
            <Text size="small" className="mt-1 text-ui-fg-muted">No promotions matched this cart.</Text>
          ) : (
            <div className="mt-2 flex flex-col gap-y-1">
              {result.adjustments.map((a) => (
                <Text key={a.promotionId} size="small" className="text-ui-fg-subtle">
                  {a.code}: -{a.amount} ({a.type})
                </Text>
              ))}
            </div>
          )}
        </div>
      )}
    </DetailPanel>
  )
}
