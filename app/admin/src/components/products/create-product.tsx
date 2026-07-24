'use client'

import { useState } from 'react'
import { Button, Heading, Input, Text, Textarea } from '@hanzo/commerce-ui'
import { useCreate } from '@/lib/api/hooks'

interface Product {
  id: string
  name: string
  slug: string
  sku: string
  description?: string
  available: boolean
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '')
}

export function CreateProduct() {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [sku, setSku] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const create = useCreate<Product>('product')

  const close = () => {
    if (create.isPending) return
    setOpen(false)
    setError('')
  }

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    const slug = slugify(name)
    const productSku = sku.trim()
    if (!slug || !productSku) {
      setError('Name and SKU are required.')
      return
    }
    setError('')
    try {
      await create.mutateAsync({
        name: name.trim(),
        slug,
        sku: productSku,
        description: description.trim(),
        available: false,
      })
      setName('')
      setSku('')
      setDescription('')
      setOpen(false)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not create the product.')
    }
  }

  return (
    <>
      <Button size="small" onClick={() => setOpen(true)}>Add product</Button>
      {open && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-ui-bg-overlay p-4" role="presentation" onMouseDown={close}>
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-product-title"
            className="w-full max-w-lg rounded-xl border border-ui-border-base bg-ui-bg-subtle p-6 shadow-elevation-modal"
            onMouseDown={(event) => event.stopPropagation()}
          >
            <Heading id="create-product-title" level="h2">Add product</Heading>
            <Text size="small" className="mt-1 text-ui-fg-muted">
              Start with the essentials. You can add media, variants, pricing, and inventory next.
            </Text>
            <form className="mt-6 space-y-4" onSubmit={submit}>
              <label className="block">
                <Text size="xsmall" weight="plus" className="mb-1.5 text-ui-fg-muted">Name</Text>
                <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus placeholder="Premium coffee beans" />
              </label>
              <label className="block">
                <Text size="xsmall" weight="plus" className="mb-1.5 text-ui-fg-muted">SKU</Text>
                <Input value={sku} onChange={(event) => setSku(event.target.value)} placeholder="COFFEE-001" />
              </label>
              <label className="block">
                <Text size="xsmall" weight="plus" className="mb-1.5 text-ui-fg-muted">Description</Text>
                <Textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={4} placeholder="What makes this product worth buying?" />
              </label>
              {error && <Text size="small" className="text-ui-fg-error">{error}</Text>}
              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" size="small" variant="secondary" onClick={close}>Cancel</Button>
                <Button type="submit" size="small" disabled={create.isPending}>
                  {create.isPending ? 'Creating…' : 'Create product'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
