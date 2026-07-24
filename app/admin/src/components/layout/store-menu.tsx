'use client'

import { useState } from 'react'
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
      <div className="flex items-center gap-2">
        <label className="min-w-0 flex-1">
          <Text size="xsmall" className="mb-1 block text-ui-fg-muted">Store</Text>
          <select
            value={current?.id || ''}
            onChange={(event) => select(event.target.value)}
            className="h-9 w-full rounded-md border border-ui-border-base bg-ui-bg-field px-2 text-sm text-ui-fg-base"
          >
            {stores.map((store) => <option key={store.id} value={store.id}>{store.name}</option>)}
          </select>
        </label>
        <Button className="mt-5" size="small" variant="secondary" onClick={() => setOpen(true)} aria-label="Add store">
          +
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
