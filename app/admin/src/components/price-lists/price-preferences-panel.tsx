'use client'

// Global price preferences (/v1/pricepreference) — per-attribute tax-inclusive
// rules that apply across price lists. Surfaced once, on the price-lists index,
// since they are store-global rather than per-list. Add + remove; the list is
// the source of truth.

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Badge, Button, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { useList, useCreate, useDelete } from '@/lib/api/hooks'
import { DetailPanel } from '@/components/common/detail-panel'
import { DeleteButton } from '@/components/common/delete-button'
import { TextField } from '@/components/common/form-fields'
import { SwitchField } from '@/components/common/choice-fields'
import {
  preferenceSchema,
  preferenceToPayload,
  emptyPreference,
  type PricePreference,
  type PreferenceValues,
} from '@/lib/price-lists/price-list'

export function PricePreferencesPanel() {
  const { data, isLoading } = useList<PricePreference>('pricepreference', { display: 200 })
  const create = useCreate<PricePreference>('pricepreference')
  const remove = useDelete('pricepreference')

  const preferences = data?.models ?? []

  const { control, handleSubmit, reset } = useForm<PreferenceValues>({
    defaultValues: emptyPreference,
    resolver: zodResolver(preferenceSchema),
  })

  const add = async (values: PreferenceValues) => {
    try {
      await create.mutateAsync(preferenceToPayload(values))
      toast.success('Preference added')
      reset(emptyPreference)
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not add the preference')
    }
  }

  const del = async (id: string) => {
    try {
      await remove.mutateAsync(id)
      toast.success('Preference removed')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not remove the preference')
    }
  }

  return (
    <DetailPanel
      title="Price preferences"
      description="Store-wide tax-inclusive pricing rules by attribute (e.g. currency_code = usd)."
    >
      {isLoading ? (
        <Skeleton className="h-16 w-full rounded-md" />
      ) : preferences.length === 0 ? (
        <Text size="small" className="text-ui-fg-muted">No preferences yet.</Text>
      ) : (
        <div className="flex flex-col divide-y divide-ui-border-base">
          {preferences.map((p) => (
            <div key={p.id} className="flex items-center justify-between gap-x-3 py-2.5 first:pt-0">
              <div className="min-w-0">
                <Text size="small" weight="plus" className="text-ui-fg-base">
                  {p.attribute} = {p.value}
                </Text>
              </div>
              <div className="flex items-center gap-x-2">
                <Badge size="2xsmall" color={p.isTaxInclusive ? 'green' : 'grey'}>
                  {p.isTaxInclusive ? 'Tax inclusive' : 'Tax exclusive'}
                </Badge>
                <DeleteButton
                  onDelete={() => del(p.id)}
                  loading={remove.isPending}
                  label="Remove"
                  title="Remove preference?"
                  description="This removes the price preference."
                />
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="mt-2 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <TextField control={control} name="attribute" label="Attribute" placeholder="currency_code" />
        <TextField control={control} name="value" label="Value" placeholder="usd" />
      </div>
      <SwitchField
        control={control}
        name="isTaxInclusive"
        label="Tax inclusive"
        description="Prices matching this attribute already include tax."
      />
      <div className="flex justify-end">
        <Button type="button" size="small" isLoading={create.isPending} onClick={handleSubmit(add)}>
          Add preference
        </Button>
      </div>
    </DetailPanel>
  )
}
