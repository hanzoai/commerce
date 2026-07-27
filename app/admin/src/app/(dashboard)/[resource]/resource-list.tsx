'use client'

import { useCallback, useMemo } from 'react'
import { useOrganizations } from '@hanzo/iam/react'
import { CommerceResource } from '@hanzo/ui/product'

import { list } from '@/lib/commerce'
import { toColumn } from '@/lib/columns'
import { resourceBySlug, type Row } from '@/lib/resources'

/** Binds one catalog row to the shared list surface for the signed-in org. */
export function ResourceList({ slug }: { slug: string }) {
  const { currentOrgId } = useOrganizations()
  const spec = resourceBySlug(slug)
  const kind = spec?.kind ?? ''
  const load = useCallback(() => list<Row>(kind, currentOrgId), [kind, currentOrgId])
  const columns = useMemo(() => (spec?.columns ?? []).map(toColumn), [spec])
  if (!spec) return null

  return (
    <CommerceResource<Row>
      title={spec.label}
      subtitle={spec.subtitle}
      load={load}
      columns={columns}
      rowKey={(r) => String(r.id ?? r.code ?? r.slug ?? JSON.stringify(r))}
      empty={spec.empty}
      hint={`endpoint · GET /v1/${spec.kind}`}
    />
  )
}
