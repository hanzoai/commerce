'use client'

import Link from 'next/link'
import { createColumnHelper } from '@tanstack/react-table'
import { Badge, Button, toast } from '@hanzo/commerce-ui'
import { DataTableShell } from '@/components/common/data-table-shell'
import { useDelete } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import type { Currency } from '@/lib/currency'

// RemoveButton disables a currency the store accepts by deleting its reference
// row. "Enable" is the inverse — create the currency (top-right action).
function RemoveButton({ id, code }: { id: string; code: string }) {
  const del = useDelete('currency')
  return (
    <Button
      size="small"
      variant="secondary"
      disabled={del.isPending}
      onClick={async (e) => {
        e.stopPropagation()
        try {
          await del.mutateAsync(id)
          toast.success(`${code.toUpperCase()} disabled`)
        } catch (err) {
          toast.error(errorMessage(err, 'Could not disable currency'))
        }
      }}
    >
      Disable
    </Button>
  )
}

const col = createColumnHelper<Currency>()

const columns = [
  col.accessor('code', {
    header: 'Code',
    cell: (info) => <span className="font-medium uppercase text-ui-fg-base">{info.getValue() || '-'}</span>,
  }),
  col.accessor('name', {
    header: 'Name',
    cell: (info) => <span className="text-ui-fg-subtle">{info.getValue() || '-'}</span>,
  }),
  col.accessor('symbol', {
    header: 'Symbol',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() || '-'}</span>,
  }),
  col.accessor('decimalDigits', {
    header: 'Decimals',
    cell: (info) => <span className="text-ui-fg-muted">{info.getValue() ?? 2}</span>,
  }),
  col.accessor('includesTax', {
    header: 'Tax',
    cell: (info) =>
      info.getValue() ? <Badge color="blue">Inclusive</Badge> : <span className="text-ui-fg-muted">Exclusive</span>,
  }),
  col.display({
    id: 'actions',
    header: '',
    cell: (info) => (
      <div className="flex justify-end">
        <RemoveButton id={info.row.original.id} code={info.row.original.code} />
      </div>
    ),
  }),
]

export default function CurrenciesPage() {
  return (
    <DataTableShell<Currency>
      kind="currency"
      title="Currencies"
      description="Enable or disable which currencies your store accepts"
      columns={columns}
      actions={
        <Link href="/currencies/create">
          <Button size="small" variant="primary">Enable currency</Button>
        </Link>
      }
    />
  )
}
