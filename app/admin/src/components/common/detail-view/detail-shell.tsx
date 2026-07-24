'use client'

// Shared chrome for a resource detail/edit page: a back link + title + subtitle +
// an actions slot, and the loading / not-found states. Both the customer and the
// customer-group detail pages render through this, so a resource page is just its
// sections. Fast first paint: a skeleton shows immediately while the record loads.
import Link from 'next/link'
import { ArrowLongLeft } from '@hanzo/commerce-icons'
import { Heading, Text } from '@hanzo/commerce-ui'
import { SingleColumnPageSkeleton } from '@/components/common/skeleton'

interface DetailShellProps {
  title: string
  subtitle?: string
  backHref: string
  backLabel?: string
  actions?: React.ReactNode
  isLoading?: boolean
  notFound?: boolean
  notFoundLabel?: string
  children: React.ReactNode
}

export function DetailShell({
  title,
  subtitle,
  backHref,
  backLabel = 'Back',
  actions,
  isLoading,
  notFound,
  notFoundLabel = 'Not found',
  children,
}: DetailShellProps) {
  return (
    <div>
      <div className="border-b border-ui-border-base px-8 py-6">
        <Link
          href={backHref}
          className="mb-2 inline-flex items-center gap-1 text-ui-fg-muted transition-colors hover:text-ui-fg-base"
        >
          <ArrowLongLeft className="h-4 w-4" />
          <Text as="span" size="small" leading="compact">{backLabel}</Text>
        </Link>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <Heading level="h1" className="truncate">{isLoading ? ' ' : title}</Heading>
            {subtitle && !isLoading && (
              <Text size="small" leading="compact" className="mt-1 text-ui-fg-subtle">{subtitle}</Text>
            )}
          </div>
          {!isLoading && !notFound && actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      </div>

      <div className="p-8">
        {isLoading ? (
          <div className="mx-auto max-w-3xl">
            <SingleColumnPageSkeleton sections={2} />
          </div>
        ) : notFound ? (
          <div className="mx-auto flex max-w-3xl flex-col items-start gap-3">
            <Text className="text-ui-fg-subtle">{notFoundLabel}</Text>
            <Link href={backHref} className="text-ui-fg-interactive hover:text-ui-fg-interactive-hover">
              <Text as="span" size="small">{backLabel}</Text>
            </Link>
          </div>
        ) : (
          <div className="mx-auto flex max-w-3xl flex-col gap-y-3">{children}</div>
        )}
      </div>
    </div>
  )
}
