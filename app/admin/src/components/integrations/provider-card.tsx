'use client'

import { Badge, Button, Container, Heading, Switch, Text } from '@hanzo/commerce-ui'
import type { Provider } from '@/lib/integrations/catalog'
import type { CommerceIntegration } from '@/lib/api/data-provider'

type Status = 'connected' | 'paused' | 'managed' | 'available'

function statusOf(provider: Provider, integration?: CommerceIntegration): Status {
  if (integration) return integration.enabled ? 'connected' : 'paused'
  if (provider.managed) return 'managed'
  return 'available'
}

const badge: Record<Status, { label: string; color: 'green' | 'orange' | 'grey' | 'blue' }> = {
  connected: { label: 'Connected', color: 'green' },
  paused: { label: 'Paused', color: 'orange' },
  managed: { label: 'Managed', color: 'blue' },
  available: { label: 'Available', color: 'grey' },
}

export interface ProviderCardProps {
  provider: Provider
  integration?: CommerceIntegration
  busy?: boolean
  onConfigure: (provider: Provider, integration?: CommerceIntegration) => void
  onToggle: (provider: Provider, integration: CommerceIntegration, enabled: boolean) => void
}

export function ProviderCard({ provider, integration, busy, onConfigure, onToggle }: ProviderCardProps) {
  const status = statusOf(provider, integration)
  const meta = badge[status]

  return (
    <Container className="flex min-h-52 flex-col p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg bg-ui-bg-component text-2xl" aria-hidden>
            {provider.emoji}
          </div>
          <div>
            <Text size="xsmall" className="text-ui-fg-muted">{provider.group}</Text>
            <Heading level="h3" className="mt-0.5">{provider.name}</Heading>
          </div>
        </div>
        <Badge color={meta.color}>{meta.label}</Badge>
      </div>

      <Text size="small" className="mt-3 flex-1 text-ui-fg-muted">{provider.note}</Text>

      <div className="mt-5 flex items-center justify-between gap-3">
        {integration ? (
          <>
            <div className="flex items-center gap-2">
              <Switch
                checked={integration.enabled}
                disabled={busy}
                onCheckedChange={(next) => onToggle(provider, integration, next)}
              />
              <Text size="small" className="text-ui-fg-subtle">
                {integration.enabled ? 'Enabled' : 'Paused'}
              </Text>
            </div>
            <Button size="small" variant="secondary" disabled={busy} onClick={() => onConfigure(provider, integration)}>
              Configure
            </Button>
          </>
        ) : provider.managed ? (
          <>
            <Text size="xsmall" className="text-ui-fg-muted">Secured in Hanzo KMS.</Text>
            <Button size="small" variant="secondary" disabled={busy} onClick={() => onConfigure(provider)}>
              Configure
            </Button>
          </>
        ) : (
          <Button size="small" disabled={busy} onClick={() => onConfigure(provider)}>
            Enable
          </Button>
        )}
      </div>
    </Container>
  )
}
