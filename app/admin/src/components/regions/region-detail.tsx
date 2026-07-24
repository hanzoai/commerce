'use client'

// Detail + edit view for one region. Reads its id from the route params, fetches
// the record client-side, and renders the shared <RegionForm> pre-filled — plus
// a countries panel driven by the region's sub-routes
// (GET/POST/DELETE /v1/region/:id/countries).

import { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useOrganizations } from '@hanzo/iam/react'
import { Badge, Button, Container, Input, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import {
  useGet,
  useUpdate,
  useDelete,
  useResourceActionData,
  useResourceAction,
} from '@/lib/api/hooks'
import { deleteOne } from '@/lib/api/data-provider'
import { PageHeader } from '@/components/common/page-header'
import { DetailPanel } from '@/components/common/detail-panel'
import { DeleteButton } from '@/components/common/delete-button'
import { RegionForm } from './region-form'
import {
  toPayload,
  toValues,
  countryPayload,
  type Region,
  type RegionValues,
  type Country,
} from '@/lib/regions/region'

function paramId(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value
}

function LoadingState() {
  return (
    <div>
      <PageHeader title="Region" description="Loading…" />
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

export function RegionDetail() {
  const params = useParams()
  const id = paramId(params?.id as string | string[] | undefined)
  const router = useRouter()

  const { data, isLoading } = useGet<Region>('region', id)
  const update = useUpdate<Region>('region')
  const remove = useDelete('region')

  if (isLoading || !id) return <LoadingState />

  if (!data) {
    return (
      <div>
        <PageHeader title="Region" description="This region could not be found." />
        <div className="p-8">
          <Text
            size="small"
            className="cursor-pointer text-ui-fg-interactive"
            onClick={() => router.push('/regions')}
          >
            Back to regions
          </Text>
        </div>
      </div>
    )
  }

  const onSubmit = async (values: RegionValues) => {
    try {
      await update.mutateAsync({ id, data: toPayload(values) })
      toast.success('Region updated')
      router.push('/regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not update the region')
    }
  }

  const onDelete = async () => {
    try {
      await remove.mutateAsync(id)
      toast.success('Region deleted')
      router.push('/regions')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not delete the region')
    }
  }

  return (
    <RegionForm
      title={data.name || 'Region'}
      description="Edit this region's settings and the countries it covers."
      submitLabel="Save changes"
      defaultValues={toValues(data)}
      submitting={update.isPending}
      onSubmit={onSubmit}
      onDelete={onDelete}
      deleting={remove.isPending}
      extra={<CountriesPanel regionId={id} />}
    />
  )
}

// ── Countries panel (GET/POST/DELETE /v1/region/:id/countries) ────────────────

function CountriesPanel({ regionId }: { regionId: string }) {
  const { currentOrgId } = useOrganizations()
  const qc = useQueryClient()
  const regionKey = [currentOrgId ?? '__no_org__', 'region']

  const { data: countries, isLoading } = useResourceActionData<Country[]>('region', regionId, 'countries')
  const add = useResourceAction<Region, Partial<Country>>('region', regionId, 'countries')
  const remove = useMutation({
    mutationFn: (iso2: string) => deleteOne(`region/${regionId}/countries`, iso2, currentOrgId),
    onSuccess: () => qc.invalidateQueries({ queryKey: regionKey }),
  })

  const [iso2, setIso2] = useState('')

  const onAdd = async () => {
    const code = iso2.trim().toLowerCase()
    if (!code) return
    try {
      await add.mutateAsync(countryPayload(code))
      toast.success('Country added')
      setIso2('')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not add the country')
    }
  }

  const onRemove = async (code: string) => {
    try {
      await remove.mutateAsync(code)
      toast.success('Country removed')
    } catch (cause) {
      toast.error(cause instanceof Error ? cause.message : 'Could not remove the country')
    }
  }

  const rows = countries ?? []

  return (
    <DetailPanel title="Countries" description="The countries this region serves.">
      {isLoading ? (
        <Skeleton className="h-16 w-full rounded-md" />
      ) : rows.length === 0 ? (
        <Text size="small" className="text-ui-fg-muted">No countries yet.</Text>
      ) : (
        <div className="flex flex-wrap gap-2">
          {rows.map((country) => (
            <Badge key={country.iso2} size="small" color="grey" className="flex items-center gap-x-1.5">
              {(country.displayName || country.name || country.iso2 || '').toString()}
              <button
                type="button"
                className="text-ui-fg-muted hover:text-ui-fg-base"
                disabled={remove.isPending}
                onClick={() => onRemove(country.iso2)}
                aria-label={`Remove ${country.iso2}`}
              >
                ×
              </button>
            </Badge>
          ))}
        </div>
      )}

      <div className="mt-2 flex items-end gap-x-3">
        <label className="flex flex-1 flex-col gap-y-1 sm:max-w-xs">
          <Text size="small" weight="plus">Country code</Text>
          <Input
            value={iso2}
            onChange={(e) => setIso2(e.target.value)}
            placeholder="us"
            maxLength={2}
          />
        </label>
        <Button type="button" size="small" isLoading={add.isPending} onClick={onAdd}>
          Add country
        </Button>
      </div>
    </DetailPanel>
  )
}
