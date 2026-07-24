import { z } from 'zod'
import type { FieldSpec } from '@/components/forms/resource-form/field-row'
import type { ResourceDescriptor } from '@/components/resource/descriptor'

// Domain module for the publishable API key resource (/v1/publishableapikey).
// Mirrors the Go model (camelCase: title, token, redacted, revokedAt, lastUsedAt).
// The full `token` is returned ONCE at create time and never again; thereafter the
// server exposes only `redacted`. A key is retired via POST /:id/revoke (soft —
// it stays listed with a `revokedAt`), not deleted.

export interface PublishableApiKey {
  id: string
  title: string
  token?: string
  redacted?: string
  revokedAt?: string
  lastUsedAt?: string
  createdAt?: string
  updatedAt?: string
}

const schema = z.object({
  title: z.string().trim().min(1, 'Title is required'),
})

export type PublishableApiKeyValues = z.infer<typeof schema>

const fields: FieldSpec<PublishableApiKeyValues>[] = [
  { name: 'title', label: 'Title', placeholder: 'Storefront' },
]

export const apiKeyDescriptor: ResourceDescriptor<PublishableApiKey, PublishableApiKeyValues> = {
  kind: 'publishableapikey',
  label: 'API key',
  listPath: '/api-keys',
  schema,
  empty: { title: '' },
  fields,
  toValues: (r) => ({ title: r.title ?? '' }),
  toPayload: (v) => ({ title: v.title.trim() }),
  recordTitle: (r) => r.title || 'API key',
  deleteDescription:
    'This permanently deletes the key. Any storefront still using it will stop working immediately.',
}

/** Mask a token for display: keep the first four and last two characters. */
export function redactToken(token?: string): string {
  if (!token) return ''
  if (token.length <= 6) return token
  return `${token.slice(0, 4)}${'•'.repeat(token.length - 6)}${token.slice(-2)}`
}
