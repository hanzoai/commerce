'use client'

// The key panel on an API key's detail surface: its redacted token, usage, and the
// revoke control (POST /v1/publishableapikey/:id/revoke — a soft retire that keeps
// the key listed with a revokedAt). Rendered via the generic <ResourceEdit> `extra`
// slot below the editable title.
import { Badge, Button, Text, toast, usePrompt } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import { useResourceAction } from '@/lib/api/hooks'
import { redactToken, type PublishableApiKey } from '@/lib/api-key'
import { errorMessage } from '@/lib/forms/schema'

export function ApiKeyPanel({ apiKey }: { apiKey: PublishableApiKey }) {
  const prompt = usePrompt()
  const revoke = useResourceAction<PublishableApiKey, void>('publishableapikey', apiKey.id, 'revoke')
  const revoked = !!apiKey.revokedAt

  const onRevoke = async () => {
    const ok = await prompt({
      title: 'Revoke API key',
      description: `Revoke "${apiKey.title}"? Any storefront using it stops working immediately.`,
      confirmText: 'Revoke',
      cancelText: 'Cancel',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await revoke.mutateAsync()
      toast.success('API key revoked')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not revoke key'))
    }
  }

  const action = revoked ? (
    <Badge size="2xsmall">Revoked</Badge>
  ) : (
    <Button size="small" variant="danger" onClick={onRevoke} isLoading={revoke.isPending}>
      Revoke
    </Button>
  )

  return (
    <Section title="Key" action={action}>
      <SectionRow
        title="Token"
        value={
          <Text family="mono" size="small">
            {apiKey.redacted || redactToken(apiKey.token) || '—'}
          </Text>
        }
      />
      <SectionRow title="Last used" value={apiKey.lastUsedAt ? new Date(apiKey.lastUsedAt).toLocaleString() : 'Never'} />
      <SectionRow title="Created" value={apiKey.createdAt ? new Date(apiKey.createdAt).toLocaleDateString() : '—'} />
    </Section>
  )
}
