'use client'

// Detail + edit view for one tax region. Reads its id from the route params,
// fetches the record client-side, and renders the shared <TaxRegionForm>
// pre-filled — plus two panels: the region's tax rates (add/remove) and a live
// tax calculate preview (POST /v1/tax/calculate).

import { useMemo, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Badge, Button, Container, Input, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useGet, useUpdate, useDelete, useList, useCreate } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'
import { DetailPanel } from '@/components/common/detail-panel'
import { DeleteButton } from '@/components/common/delete-button'
import { TextField } from '@/components/common/form-fields'
import { SwitchField } from '@/components/common/choice-fields'
import { TaxRegionForm } from './tax-region-form'
import {
  toPayload,
  toValues,
  taxRegionName,
  taxRateSchema,
  taxRateToPayload,
  emptyTaxRate,
  type TaxRegion,
  type TaxRegionValues,
  type TaxRate,
  type TaxRateValues,
  type TaxCalcResponse,
} from '@/lib/tax-regions/tax-region'

function paramId(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Tax region" description="Loading…" />
      <div className="p-8">
        <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-md" />
          ))}
        </Container>
      </div>
    </div>
  )
}

export function TaxRegionDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<TaxRegion>('taxregion', id)
  const update = useUpdate<TaxRegion>('taxregion')
  const remove = useDelete('taxregion')

  if (isLoading || !id) return <LoadingState />

  if (!data) {
    return (
      <div>
        <PageHeader title="Tax region" description="This tax region could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/tax-regions')}
          >
            Back to tax regions
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: TaxRegionValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Tax region updated')
      router.push('/tax-regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the tax region')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Tax region deleted')
      router.push('/tax-regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the tax region')
    }
  }

  return (
    <TaxRegionForm
      title={taxRegionName(data)}
      description="Edit this tax region, its rates, and preview tax on a sample cart."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      extra={
        <div className="flex flex-col gap-y-6">
          <RatesPanel taxRegionId={id} />
          <CalculatePanel country={data.countryCode} province={data.provinceCode ?? ''} />
        </div>
      }
    />
  )
}

// ── Tax rates panel ───────────────────────────────────────────────────────────

function RatesPanel({ taxRegionId }: { taxRegionId: string }) {
  const { data, isLoading } = useList<TaxRate>('taxrate', { display: 200 })
  const create = useCreate<TaxRate>('taxrate')
  const remove = useDelete('taxrate')

  const rates = useMemo(
    () => (data?.models ?? []).filter((r) => r.taxRegionId === taxRegionId),
    [data, taxRegionId],
  )

  const { control, handleSubmit, reset } = useForm<TaxRateValues>({
    defaultValues: emptyTaxRate,
    resolver: zodResolver(taxRateSchema),
  })

  const add = async (values: TaxRateValues) => {
    try {
      await create.mutateAsync(taxRateToPayload(values, taxRegionId))
      toast.success('Tax rate added')
      reset(emptyTaxRate)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not add the tax rate')
    }
  }

  const del = async (id: string) => {
    try {
      await remove.mutateAsync(id)
      toast.success('Tax rate removed')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not remove the tax rate')
    }
  }

  return (
    <DetailPanel title="Tax rates" description="The rates that apply in this region.">
      {isLoading ? (
        <Skeleton className="h-20 w-full rounded-md" />
      ) : rates.length === 0 ? (
        <Text size="small" className="text-ui-fg-muted">No tax rates yet.</Text>
      ) : (
        <div className="flex flex-col divide-y divide-ui-border-base">
          {rates.map((r) => (
            <div key={r.id} className="flex items-center justify-between gap-x-3 py-2.5 first:pt-0">
              <div className="min-w-0">
                <Text size="small" weight="plus" className="text-ui-fg-base">
                  {r.name} · {r.rate}%
                </Text>
                {r.code && <Text size="xsmall" className="text-ui-fg-muted">{r.code}</Text>}
              </div>
              <div className="flex items-center gap-x-2">
                {r.isDefault && <Badge size="2xsmall" color="blue">Default</Badge>}
                {r.isCombinable && <Badge size="2xsmall" color="grey">Combinable</Badge>}
                <DeleteButton
                  onDelete={() => del(r.id)}
                  loading={remove.isPending}
                  label="Remove"
                  title="Remove tax rate?"
                  description="This removes the rate from the region."
                />
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="mt-2 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <TextField control={control} name="name" label="Name" placeholder="Standard rate" />
        <TextField control={control} name="rate" label="Rate (%)" placeholder="8.5" />
        <TextField control={control} name="code" label="Code" optional placeholder="STD" className="sm:col-span-2" />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SwitchField control={control} name="isDefault" label="Default" description="The region's default rate." />
        <SwitchField control={control} name="isCombinable" label="Combinable" description="Stacks with other rates." />
      </div>
      <div className="flex justify-end">
        <Button type="button" size="small" isLoading={create.isPending} onClick={handleSubmit(add)}>
          Add rate
        </Button>
      </div>
    </DetailPanel>
  )
}

// ── Calculate preview panel ───────────────────────────────────────────────────

function CalculatePanel({ country, province }: { country: string; province: string }) {
  const calc = useCreate<TaxCalcResponse>('tax/calculate')
  const [amount, setAmount] = useState('100')
  const [quantity, setQuantity] = useState('1')
  const [result, setResult] = useState<TaxCalcResponse | null>(null)

  const run = async () => {
    try {
      const res = await calc.mutateAsync({
        items: [{ amount: Number(amount) || 0, quantity: Number(quantity) || 1 }],
        shippingAddress: { countryCode: country, provinceCode: province },
      } as any)
      setResult(res)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Calculate failed')
    }
  }

  return (
    <DetailPanel
      title="Preview"
      description="Compute tax for a sample item shipped to this region."
      action={
        <Button type="button" size="small" variant="secondary" isLoading={calc.isPending} onClick={run}>
          Calculate
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Item amount</Text>
          <Input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" placeholder="100" />
        </label>
        <label className="flex flex-col gap-y-1">
          <Text size="small" weight="plus">Quantity</Text>
          <Input value={quantity} onChange={(e) => setQuantity(e.target.value)} inputMode="numeric" placeholder="1" />
        </label>
      </div>
      {result && (
        <div className="rounded-lg border border-ui-border-base p-4">
          <Text size="small" weight="plus" className="text-ui-fg-base">
            Total tax: {result.totalTax}
          </Text>
          {result.items.map((item, i) => (
            <Text key={i} size="small" className="mt-1 text-ui-fg-subtle">
              {item.amount} × {item.quantity} @ {item.taxRate}% → tax {item.tax}
            </Text>
          ))}
        </div>
      )}
    </DetailPanel>
  )
}
