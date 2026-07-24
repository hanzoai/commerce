'use client'

// The subcategories of one category, read from /v1/product-category/:id/children
// and rendered as links into their own detail pages. Shown below the general
// fields on the category detail surface via the generic <ResourceEdit> `extra` slot.
import Link from 'next/link'
import { Text } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { useList } from '@/lib/api/hooks'
import { childrenKind, type ProductCategory } from '@/lib/category'

export function CategoryChildren({ id }: { id: string }) {
  const { data, isLoading } = useList<ProductCategory>(childrenKind(id), { display: 100 })
  const children = data?.models ?? []

  return (
    <Section title="Subcategories">
      {isLoading ? (
        <div className="px-6 py-4">
          <Text size="small" className="text-ui-fg-muted">Loading…</Text>
        </div>
      ) : children.length === 0 ? (
        <div className="px-6 py-4">
          <Text size="small" className="text-ui-fg-muted">No subcategories yet.</Text>
        </div>
      ) : (
        <div className="divide-y">
          {children.map((c) => (
            <Link
              key={c.id}
              href={`/categories/${c.id}`}
              className="flex items-center justify-between px-6 py-3 transition-colors hover:bg-ui-bg-base-hover"
            >
              <Text size="small" className="text-ui-fg-base">{c.name}</Text>
              <Text size="xsmall" className="text-ui-fg-muted">{c.isActive === false ? 'Inactive' : 'Active'}</Text>
            </Link>
          ))}
        </div>
      )}
    </Section>
  )
}
