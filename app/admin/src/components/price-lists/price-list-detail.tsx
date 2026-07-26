'use client'

// Detail + edit view for one price list. Reads its id from the route params,
// fetches the record client-side, and renders the shared <PriceListForm>
// pre-filled — plus two panels below the fields: the prices on this list
// (add/remove) and a live pricing calculate preview (POST /v1/pricing/calculate).

import { useMemo, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Button, Container, Input, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useGet, useUpdate, useDelete, useList, useCreate } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'
import { DetailPanel } from '@/components/common/detail-panel'
import { DeleteButton } from '@/components/common/delete-button'
import { TextField } from '@/components/common/form-fields'
import { SelectField } from '@/components/common/choice-fields'
import { useCurrencyOptions } from '@/lib/currency'
import { PriceListForm } from './price-list-form'
import {
  toPayload,
  toValues,
  priceSchema,
  priceToPayload,
  emptyPrice,
  type PriceList,
  type PriceListValues,
  type Price,
  type PriceValues,
  type CalculateResponse,
} from '@/lib/price-lists/price-list'

function paramId(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Price list" description="Loading…" />
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

export function PriceListDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<PriceList>('pricelist', id)
  const update = useUpdate<PriceList>('pricelist')
  const remove = useDelete('pricelist')

  if (isLoading || !id) return <LoadingState />

  if (!data) {
    return (
      <div>
        <PageHeader title="Price list" description="This price list could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/price-lists')}
          >
            Back to price lists
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: PriceListValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Price list updated')
      router.push('/price-lists')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the price list')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Price list deleted')
      router.push('/price-lists')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the price list')
    }
  }

  return (
    <PriceListForm
      title={data.title || 'Price list'}
      description="Edit this price list, its prices, and preview resolved pricing."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      extra={
        <div className="flex flex-col gap-y-6">
          <PricesPanel priceListId={id} />
          <CalculatePanel />
        </div>
      }
    />
  )
}

// ── Prices panel ──────────────────────────────────────────────────────────────

function PricesPanel({ priceListId }: { priceListId: string }) {
  const { data, isLoading } = useList<Price>('price', { display: 200 })
  const create = useCreate<Price>('price')
  const remove = useDelete('price')
  const currencyOptions = useCurrencyOptions()

  const prices = useMemo(
    () => (data?.models ?? []).filter((p) => p.priceListId === priceListId),
    [data, priceListId],
  )

  const { control, handleSubmit, reset } = useForm<PriceValues>({
    defaultValues: emptyPrice,
    resolver: zodResolver(priceSchema),
  })

  const add = async (values: PriceValues) => {
    try {
      await create.mutateAsync(priceToPayload(values, priceListId))
      toast.success('Price added')
      reset(emptyPrice)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not add the price')
    }
  }

  const del = async (id: string) => {
    try {
      await remove.mutateAsync(id)
      toast.success('Price removed')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not remove the price')
    }
  }

  return (
    <DetailPanel title="Prices" description="The amounts this price list sets, by currency and quantity.">
      {isLoading ? (
        <Skeleton className="h-20 w-full rounded-md" />
      ) : prices.length === 0 ? (
        <Text size="small" className="text-ui-fg-muted">No prices yet.</Text>
      ) : (
        <div className="flex flex-col divide-y divide-ui-border-base">
          {prices.map((p) => (
            <div key={p.id} className="flex items-center justify-between gap-x-3 py-2.5 first:pt-0">
              <div className="min-w-0">
                <Text size="small" weight="plus" className="text-ui-fg-base">
                  {p.amount} {(p.currencyCode || '').toUpperCase()}
                </Text>
                <Text size="xsmall" className="text-ui-fg-muted">
                  qty {p.minQuantity || 0}{p.maxQuantity ? `–${p.maxQuantity}` : '+'}
                  {p.priceSetId ? ` · set ${p.priceSetId}` : ''}
                </Text>
              </div>
              <DeleteButton
                onDelete={() => del(p.id)}
                loading={remove.isPending}
                label="Remove"
                title="Remove price?"
                description="This removes the price from the list."
              />
            </div>
          ))}
        </div>
      )}

      <div className="mt-2 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField control={control} name="currencyCode" label="Currency" options={currencyOptions} placeholder="Select currency" />
        <TextField control={control} name="amount" label="Amount (cents)" placeholder="1999" />
        <TextField control={control} name="minQuantity" label="Min quantity" optional placeholder="1" />
        <TextField control={control} name="maxQuantity" label="Max quantity" optional placeholder="10" />
        <TextField control={control} name="priceSetId" label="Price set ID" optional placeholder="pset_…" className="sm:col-span-2" />
      </div>
      <div className="flex justify-end">
        <Button type="button" size="small" isLoading={create.isPending} onClick={handleSubmit(add)}>
          Add price
        </Button>
      </div>
    </DetailPanel>
  )
}

// ── Calculate preview panel ───────────────────────────────────────────────────

function CalculatePanel() {
  const calc = useCreate<CalculateResponse>('pricing/calculate')
  const [priceSetId, setPriceSetId] = useState('')
  const [quantity, setQuantity] = useState('1')
  const [currency, setCurrency] = useState('usd')
  const [result, setResult] = useState<CalculateResponse | null>(null)

  const run = async () => {
    try {
      const res = await calc.mutateAsync({
        currencyCode: currency,
        items: [{ priceSetId, quantity: Number(quantity) || 1 }],
      } as any)
      setResult(res)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Calculate failed')
    }
  }

  return (
    <DetailPanel
      title="Preview"
      description="Resolve the effective price for a price set + quantity."
      action={
        <Button type="button" size="small" variant="secondary" isLoading={calc.isPending} onClick={run}>
          Calculate
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Price set ID</Text>
          <Input value={priceSetId} onChange={(e) => setPriceSetId(e.target.value)} placeholder="pset_…" />
        </label>
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Quantity</Text>
          <Input value={quantity} onChange={(e) => setQuantity(e.target.value)} inputMode="numeric" placeholder="1" />
        </label>
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Currency</Text>
          <Input value={currency} onChange={(e) => setCurrency(e.target.value)} placeholder="usd" />
        </label>
      </div>
      {result && (
        <div className="rounded-lg border border-ui-border-base p-4">
          {result.items.length === 0 ? (
            <Text size="small" className="text-ui-fg-muted">No matching prices.</Text>
          ) : (
            result.items.map((item, i) => (
              <Text key={i} size="small" className="text-ui-fg-subtle">
                {item.priceSetId || '(set)'}: {item.amount} {(item.currencyCode || '').toUpperCase()}
                {item.priceListId ? ` · from ${item.priceListId}` : ''}
              </Text>
            ))
          )}
        </div>
      )}
    </DetailPanel>
  )
}
