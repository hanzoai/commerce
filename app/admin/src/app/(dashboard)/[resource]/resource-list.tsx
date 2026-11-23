'use client'

import { useCallback, useMemo } from 'react'
import { CommerceResource } from '@hanzo/ui/product'

import { endpointPath, list } from '@/lib/commerce'
import { toColumn } from '@/lib/columns'
import { useActiveOrg } from '@/lib/org'
import { resourceBySlug, type Row } from '@/lib/resources'

/** Binds one catalog row to the shared list surface for the active organization. */
export function ResourceList({ slug }: { slug: string }) {
  const { currentOrgId } = useActiveOrg()
  const spec = resourceBySlug(slug)
  const kind = spec?.kind ?? ''
  const load = useCallback(() => list<Row>(kind, currentOrgId), [kind, currentOrgId])
  const columns = useMemo(() => (spec?.columns ?? []).map(toColumn), [spec])
  if (!spec) return null

  return (
    <CommerceResource<Row>
      key={`${spec.slug}:${currentOrgId ?? ''}`}
      title={spec.label}
      subtitle={`${spec.subtitle} (org: ${currentOrgId ?? 'default'})`}
      load={load}
      columns={columns}
      rowKey={(r) => String(r.id ?? r.transactionId ?? r.code ?? r.slug ?? JSON.stringify(r))}
      empty={spec.empty}
      hint={`endpoint · GET ${endpointPath(spec.kind)} (X-Org-Id: ${currentOrgId ?? 'default'})`}
    />
  )
}
