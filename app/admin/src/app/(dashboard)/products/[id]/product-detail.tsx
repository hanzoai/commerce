'use client'

import { use } from 'react'
import Link from 'next/link'
import { Button, Heading, Text, Container } from '@hanzo/commerce-ui'
import { useProduct } from '@/lib/api/hooks'
import { PageHeader } from '@/components/common/page-header'

export function ProductDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { data: product, isLoading } = useProduct(id)

  if (isLoading) {
    return (
      <div>
        <PageHeader title="Loading..." />
        <div className="p-8">
          <div className="space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-12 animate-pulse rounded-lg bg-ui-bg-component" />
            ))}
          </div>
        </div>
      </div>
    )
  }

  if (!product) {
    return (
      <div>
        <PageHeader title="Product Not Found" />
        <div className="p-8 text-center">
          <Text size="small" className="text-ui-fg-muted">This product doesn&apos;t exist or you don&apos;t have access.</Text>
          <Button variant="secondary" className="mt-4" asChild>
            <Link href="/products">Back to Products</Link>
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div>
      <PageHeader
        title={product.name || 'Untitled Product'}
        description={`ID: ${product.id}`}
        actions={
          <Button variant="secondary" asChild>
            <Link href="/products">Back to Products</Link>
          </Button>
        }
      />
      <div className="p-8">
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <Container className="p-6">
            <Heading level="h3" className="mb-4">Details</Heading>
            <dl className="space-y-3">
              <Field label="Name" value={product.name} />
              <Field label="Slug" value={product.slug} />
              <Field label="Description" value={product.description} />
              <Field label="Status" value={product.status || 'draft'} />
              <Field
                label="Price"
                value={
                  product.price
                    ? new Intl.NumberFormat('en-US', {
                        style: 'currency',
                        currency: product.currency || 'USD',
                      }).format(product.price / 100)
                    : '-'
                }
              />
            </dl>
          </Container>

          <Container className="p-6">
            <Heading level="h3" className="mb-4">Metadata</Heading>
            <dl className="space-y-3">
              <Field label="Created" value={product.createdAt ? new Date(product.createdAt).toLocaleString() : '-'} />
              <Field label="Updated" value={product.updatedAt ? new Date(product.updatedAt).toLocaleString() : '-'} />
            </dl>
          </Container>
        </div>
      </div>
    </div>
  )
}

function Field({ label, value }: { label: string; value?: string | null }) {
  return (
    <div>
      <Text as="span" size="xsmall" className="text-ui-fg-muted">{label}</Text>
      <Text size="small" className="mt-0.5 text-ui-fg-base">{value || '-'}</Text>
    </div>
  )
}
