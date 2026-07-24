'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button, Text } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { redactToken, type PublishableApiKey } from '@/lib/api-key'

const col = createColumnHelper<PublishableApiKey>()

const columns = [
  col.accessor('title', {
    header: 'Title',
    cell: (info) => <span className="font-medium text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('redacted', {
    header: 'Token',
    cell: (info) => (
      <Text family="mono" size="small" className="text-ui-fg-muted">
        {info.getValue() || redactToken(info.row.original.token) || '—'}
      </Text>
    ),
  }),
  col.accessor('revokedAt', {
    header: 'Status',
    cell: (info) =>
      info.getValue() ? <Badge size="2xsmall">Revoked</Badge> : <Badge size="2xsmall" color="green">Active</Badge>,
  }),
  col.accessor('createdAt', {
    header: 'Created',
    cell: (info) => {
      const d = info.getValue()
      return <span className="text-ui-fg-muted">{d ? new Date(d).toLocaleDateString() : '-'}</span>
    },
  }),
]

export default function ApiKeysPage() {
  return (
    <DataTableShell<PublishableApiKey>
      kind="publishableapikey"
      title="API keys"
      description="Publishable keys that authorize your storefront to read the catalog"
      columns={columns}
      detailPath="/api-keys"
      actions={
        <Link href="/api-keys/create">
          <Button size="small" variant="primary">Create</Button>
        </Link>
      }
    />
  )
}
