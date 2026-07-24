'use client'

import { useState } from 'react'
import { Button, Input, Label, Text, toast } from '@hanzo/commerce-ui'
import { useCreate, useCount } from '@/lib/api/hooks'
import { requestAiPrompt } from '@/lib/ai-bus'
import { WizardStep, StepNav } from '../wizard-step'
import type { StepProps } from './types'

function slugify(value: string): string {
  return value.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

export function ProductStep({ onNext, onBack, onSkip }: StepProps) {
  const create = useCreate('product')
  const { data: productCount = 0 } = useCount('product')
  const [name, setName] = useState('')
  const [price, setPrice] = useState('')

  const submit = async () => {
    const clean = name.trim()
    if (!clean) return
    const slug = slugify(clean) || clean
    const dollars = parseFloat(price)
    const cents = Number.isFinite(dollars) ? Math.round(dollars * 100) : undefined
    try {
      await create.mutateAsync({
        name: clean,
        slug,
        sku: slug.toUpperCase(),
        available: true,
        ...(cents != null ? { price: cents } : {}),
      })
      toast.success('Product added', { description: clean })
      onNext()
    } catch {
      toast.error('Could not add product', { description: 'Please try again.' })
    }
  }

  // Hand the whole catalog build to the AI dock — it opens pre-filled and the
  // merchant confirms/sends. `generate_catalog` creates products via the same
  // createOne("product") path this form uses.
  const generateWithAi = () => {
    requestAiPrompt('Generate a starter catalog of 6 products for my store, with names, short descriptions, and prices.')
    toast.info('Assistant ready', { description: 'Review the catalog prompt, then send.' })
  }

  return (
    <WizardStep
      title="Add your first product"
      description="Add one product to get started, or let the assistant generate a whole catalog for you."
    >
      <div className="mb-6 rounded-lg border border-dashed border-ui-border-strong bg-ui-bg-subtle p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <SparkleIcon />
            <div>
              <Text size="small" weight="plus" className="text-ui-fg-base">Generate my catalog</Text>
              <Text size="xsmall" className="text-ui-fg-muted">Let AI draft products, descriptions, and prices.</Text>
            </div>
          </div>
          <Button variant="secondary" size="small" onClick={generateWithAi}>Ask the assistant</Button>
        </div>
      </div>

      <div className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="product-name" weight="plus">Product name</Label>
          <Input
            id="product-name"
            placeholder="Cold Brew"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="product-price" weight="plus">Price (USD)</Label>
          <Input
            id="product-price"
            type="number"
            min="0"
            step="0.01"
            placeholder="6.00"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') submit()
            }}
          />
        </div>
        {productCount > 0 && (
          <Text size="small" className="text-ui-fg-muted">
            {productCount} product{productCount === 1 ? '' : 's'} in your catalog so far.
          </Text>
        )}
      </div>

      <StepNav
        onNext={submit}
        onBack={onBack}
        onSkip={onSkip}
        nextLabel="Add product"
        nextDisabled={!name.trim()}
        nextLoading={create.isPending}
      />
    </WizardStep>
  )
}

function SparkleIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="text-ui-fg-base">
      <path d="M12 3l1.9 4.6L18.5 9.5 13.9 11.4 12 16l-1.9-4.6L5.5 9.5l4.6-1.9L12 3z" />
      <path d="M19 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8.8-2z" />
    </svg>
  )
}
