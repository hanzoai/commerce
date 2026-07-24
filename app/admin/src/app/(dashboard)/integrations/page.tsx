'use client'

import { useMemo, useState } from 'react'
import dynamic from 'next/dynamic'
import { Container, Input, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ProviderCard } from '@/components/integrations/provider-card'
import { catalog, groups, type Provider } from '@/lib/integrations/catalog'
import { useIntegrations } from '@/lib/api/hooks'
import type { CommerceIntegration, IntegrationInput } from '@/lib/api/data-provider'

// The Configure/Connect drawer pulls in react-hook-form + zod — heavy and only
// needed on interaction. Code-split it so the marketplace grid paints instantly.
const ConfigureDrawer = dynamic(
  () => import('@/components/integrations/configure-drawer').then((m) => m.ConfigureDrawer),
  { ssr: false },
)

export default function IntegrationsPage() {
  const { data = [], isLoading, save } = useIntegrations()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState<{ provider: Provider; integration?: CommerceIntegration } | null>(null)

  // One lookup from provider `type` → its saved integration row.
  const byType = useMemo(() => {
    const map = new Map<string, CommerceIntegration>()
    for (const row of data) map.set(row.type, row)
    return map
  }, [data])

  // Filter once, then bucket by group preserving the catalog's group order
  // (Payments first). Memoized so typing in search never re-walks on unrelated renders.
  const sections = useMemo(() => {
    const q = query.trim().toLowerCase()
    const match = (p: Provider) =>
      !q ||
      p.name.toLowerCase().includes(q) ||
      p.group.toLowerCase().includes(q) ||
      p.note.toLowerCase().includes(q)
    return groups
      .map((group) => ({ group, providers: catalog.filter((p) => p.group === group && match(p)) }))
      .filter((section) => section.providers.length > 0)
  }, [query])

  const configure = (provider: Provider, integration?: CommerceIntegration) => setActive({ provider, integration })

  const toggle = (provider: Provider, integration: CommerceIntegration, enabled: boolean) => {
    // Flip enabled only — omit `data` so KMS credentials are left untouched.
    save.mutate({ id: integration.id, type: integration.type, enabled })
  }

  const submit = (input: IntegrationInput) => save.mutateAsync(input)

  return (
    <div>
      <PageHeader
        title="Integrations"
        description="Browse the marketplace and connect payments, fulfillment, marketing, analytics, and operations in one click"
      />
      <div className="p-8">
        <div className="mb-6 max-w-sm">
          <Input
            placeholder="Search providers…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>

        {isLoading ? (
          <CardGridSkeleton />
        ) : sections.length === 0 ? (
          <Text size="small" className="py-16 text-center text-ui-fg-muted">
            No providers match “{query}”.
          </Text>
        ) : (
          <div className="space-y-10">
            {sections.map((section) => (
              <section key={section.group}>
                <Text size="small" weight="plus" className="mb-3 text-ui-fg-subtle">
                  {section.group}
                </Text>
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                  {section.providers.map((provider) => (
                    <ProviderCard
                      key={provider.type}
                      provider={provider}
                      integration={byType.get(provider.type)}
                      busy={save.isPending}
                      onConfigure={configure}
                      onToggle={toggle}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        )}

        <Text size="xsmall" className="mt-10 text-ui-fg-muted">
          Provider secrets are encrypted in Hanzo KMS — never persisted in plaintext.
        </Text>
      </div>

      <ConfigureDrawer
        provider={active?.provider ?? null}
        integration={active?.integration}
        open={!!active}
        onOpenChange={(open) => !open && setActive(null)}
        onSubmit={submit}
        pending={save.isPending}
      />
    </div>
  )
}

function CardGridSkeleton() {
  return (
    <div className="space-y-10">
      {[0, 1].map((section) => (
        <div key={section}>
          <div className="mb-3 h-4 w-24 animate-pulse rounded bg-ui-bg-component" />
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {[...Array(3)].map((_, i) => (
              <Container key={i} className="min-h-52 p-5">
                <div className="flex items-center gap-3">
                  <div className="h-11 w-11 animate-pulse rounded-lg bg-ui-bg-component" />
                  <div className="flex-1 space-y-2">
                    <div className="h-3 w-16 animate-pulse rounded bg-ui-bg-component" />
                    <div className="h-4 w-28 animate-pulse rounded bg-ui-bg-component" />
                  </div>
                </div>
                <div className="mt-4 h-3 w-full animate-pulse rounded bg-ui-bg-component" />
                <div className="mt-2 h-3 w-2/3 animate-pulse rounded bg-ui-bg-component" />
                <div className="mt-6 h-8 w-full animate-pulse rounded bg-ui-bg-component" />
              </Container>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
