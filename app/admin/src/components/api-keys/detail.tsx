'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { apiKeyDescriptor, type PublishableApiKey } from '@/lib/api-key'
import { ApiKeyPanel } from './api-key-panel'

export function ApiKeyDetail() {
  return (
    <ResourceEdit
      descriptor={apiKeyDescriptor}
      description="Rename this key, review its usage, or revoke it."
      extra={(record: PublishableApiKey) => <ApiKeyPanel apiKey={record} />}
    />
  )
}
