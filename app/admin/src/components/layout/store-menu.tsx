'use client'

import { useState } from 'react'
import {
  BuildingStorefront,
  ChevronDownMini,
  Plus,
} from '@hanzo/commerce-icons'
import { Button, Heading, Input, Text } from '@hanzo/commerce-ui'
import { useStore, useStores } from '@/lib/api/hooks'

function slugify(value: string) {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

export function StoreMenu() {
  const { data: current } = useStore()
  const { data, create, select } = useStores()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const stores = data?.models ?? (current ? [current] : [])

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    const slug = slugify(name)
    if (!slug) return
    setError('')
    try {
      await create.mutateAsync({ name: name.trim(), slug, currency: 'usd' })
      setName('')
      setOpen(false)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not create the store.')
    }
  }

  return (
    <>
      <div className="flex min-w-0 items-center gap-2">
        <Text size="xsmall" weight="plus" className="hidden text-ui-fg-muted sm:block">
          Store
        </Text>
        <label className="relative min-w-0">
          <span className="sr-only">Switch store</span>
          <BuildingStorefront className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-ui-fg-muted" />
          <select
            value={current?.id || ''}
            onChange={(event) => select(event.target.value)}
            aria-label="Switch store"
            className="h-9 max-w-48 appearance-none truncate rounded-md border border-ui-border-base bg-ui-bg-field py-0 pl-8 pr-8 text-sm text-ui-fg-base focus:outline-none focus:ring-1 focus:ring-ui-fg-interactive sm:max-w-64"
          >
            {stores.map((store) => <option key={store.id} value={store.id}>{store.name}</option>)}
          </select>
          <ChevronDownMini className="pointer-events-none absolute right-2.5 top-3 h-3 w-3 text-ui-fg-muted" />
        </label>
        <Button size="small" variant="secondary" onClick={() => setOpen(true)} aria-label="Create store">
          <Plus className="h-4 w-4" />
        </Button>
      </div>

      {open && (
        <div className="fixed inset-0 z-[80] flex items-center justify-center bg-ui-bg-overlay p-4" onMouseDown={() => setOpen(false)}>
          <div className="w-full max-w-md rounded-xl border border-ui-border-base bg-ui-bg-subtle p-6 shadow-elevation-modal" onMouseDown={(event) => event.stopPropagation()}>
            <Heading level="h2">Create store</Heading>
            <Text size="small" className="mt-1 text-ui-fg-muted">Each store has its own trial and $20 monthly plan.</Text>
            <form className="mt-5 space-y-4" onSubmit={submit}>
              <label className="block">
                <Text size="xsmall" weight="plus" className="mb-1.5 text-ui-fg-muted">Store name</Text>
                <Input autoFocus value={name} onChange={(event) => setName(event.target.value)} placeholder="Coffee House" />
              </label>
              {error && <Text size="small" className="text-ui-fg-error">{error}</Text>}
              <div className="flex justify-end gap-2">
                <Button type="button" size="small" variant="secondary" onClick={() => setOpen(false)}>Cancel</Button>
                <Button type="submit" size="small" disabled={!slugify(name) || create.isPending}>
                  {create.isPending ? 'Creating…' : 'Create store'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
