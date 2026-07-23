'use client'

import { useMemo, useState } from 'react'
import { Text, Container, Badge, Input } from '@hanzo/commerce-ui'
import { useModels } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'

function price(n?: number) {
  if (n == null) return '—'
  return `$${n.toFixed(2)}`
}

export default function ModelsPage() {
  const { data, isLoading } = useModels()
  const [q, setQ] = useState('')
  const models = data ?? []

  const filtered = useMemo(() => {
    const s = q.trim().toLowerCase()
    const rows = s
      ? models.filter((m) => m.id.toLowerCase().includes(s) || (m.provider || m.owned_by || '').toLowerCase().includes(s))
      : models
    return [...rows].sort(
      (a, b) =>
        Number(!!b.premium) - Number(!!a.premium) ||
        (b.pricing?.input ?? 0) - (a.pricing?.input ?? 0) ||
        a.id.localeCompare(b.id),
    )
  }, [models, q])

  const premiumCount = models.filter((m) => m.premium).length

  return (
    <div>
      <PageHeader title="Models" description="Your model catalog and per-model pricing (USD per 1M tokens)" />
      <div className="p-8">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <Text size="small" className="text-ui-fg-muted">
            {isLoading ? 'Loading models…' : `${models.length} model${models.length === 1 ? '' : 's'} · ${premiumCount} premium`}
          </Text>
          <Input placeholder="Search models or provider…" value={q} onChange={(e) => setQ(e.target.value)} className="w-full sm:w-72" />
        </div>

        <Container className="overflow-hidden p-0">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[640px]">
              <thead>
                <tr className="border-b border-ui-border-base text-left">
                  <th className="px-4 py-3"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Model</Text></th>
                  <th className="px-4 py-3"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Provider</Text></th>
                  <th className="px-4 py-3"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Tier</Text></th>
                  <th className="px-4 py-3 text-right"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Input / 1M</Text></th>
                  <th className="px-4 py-3 text-right"><Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">Output / 1M</Text></th>
                </tr>
              </thead>
              <tbody>
                {isLoading ? (
                  [...Array(8)].map((_, i) => (
                    <tr key={i}><td colSpan={5} className="px-4 py-2"><div className="h-8 animate-pulse rounded bg-ui-bg-component" /></td></tr>
                  ))
                ) : filtered.length === 0 ? (
                  <tr><td colSpan={5}><Text size="small" className="block py-10 text-center text-ui-fg-muted">{q ? `No models match “${q}”` : 'No models available for this organization'}</Text></td></tr>
                ) : (
                  filtered.map((m) => (
                    <tr key={m.id} className="border-b border-ui-border-base last:border-0 hover:bg-ui-bg-base-hover">
                      <td className="px-4 py-3"><Text as="span" size="small" weight="plus" className="text-ui-fg-base">{m.id}</Text></td>
                      <td className="px-4 py-3"><Text as="span" size="small" className="text-ui-fg-muted">{m.provider || m.owned_by || '—'}</Text></td>
                      <td className="px-4 py-3">
                        {m.premium ? <Badge color="purple" size="2xsmall">Premium</Badge> : <Badge color="grey" size="2xsmall">Standard</Badge>}
                      </td>
                      <td className="px-4 py-3 text-right"><Text as="span" size="small" className="tabular-nums text-ui-fg-base">{price(m.pricing?.input)}</Text></td>
                      <td className="px-4 py-3 text-right"><Text as="span" size="small" className="tabular-nums text-ui-fg-base">{price(m.pricing?.output)}</Text></td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </Container>
      </div>
    </div>
  )
}
