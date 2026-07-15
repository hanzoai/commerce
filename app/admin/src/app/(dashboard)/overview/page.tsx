'use client'

import { Heading, Text, Container, Badge } from '@hanzo/commerce-ui'
import { useCount, useStore } from '@/lib/api/hooks'
import type { StoreListing } from '@/lib/api/data-provider'
import { StatCard } from '@/components/common/stat-card'
import { PageHeader } from '@/components/common/page-header'

export default function DashboardPage() {
  const { data: productCount, isLoading: loadingProducts } = useCount('product')
  const { data: orderCount, isLoading: loadingOrders } = useCount('order')
  const { data: customerCount, isLoading: loadingCustomers } = useCount('c/user')
  const { data: collectionCount, isLoading: loadingCollections } = useCount('collection')

  return (
    <div>
      <PageHeader title="Dashboard" description="Overview of your commerce operations" />
      <div className="p-8">
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Products"
            value={productCount ?? 0}
            loading={loadingProducts}
            icon={
              <svg className="h-5 w-5 text-ui-fg-subtle" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
              </svg>
            }
          />
          <StatCard
            label="Orders"
            value={orderCount ?? 0}
            loading={loadingOrders}
            icon={
              <svg className="h-5 w-5 text-ui-fg-subtle" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 10.5V6a3.75 3.75 0 1 0-7.5 0v4.5m11.356-1.993 1.263 12c.07.665-.45 1.243-1.119 1.243H4.25a1.125 1.125 0 0 1-1.12-1.243l1.264-12A1.125 1.125 0 0 1 5.513 7.5h12.974c.576 0 1.059.435 1.119 1.007ZM8.625 10.5a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm7.5 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
              </svg>
            }
          />
          <StatCard
            label="Customers"
            value={customerCount ?? 0}
            loading={loadingCustomers}
            icon={
              <svg className="h-5 w-5 text-ui-fg-subtle" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" />
              </svg>
            }
          />
          <StatCard
            label="Collections"
            value={collectionCount ?? 0}
            loading={loadingCollections}
            icon={
              <svg className="h-5 w-5 text-ui-fg-subtle" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9.568 3H5.25A2.25 2.25 0 0 0 3 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 0 0 5.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 0 0 9.568 3Z" />
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 6h.008v.008H6V6Z" />
              </svg>
            }
          />
        </div>

        <StoreOverview />

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <RecentOrders />
          <TopProducts />
        </div>
      </div>
    </div>
  )
}

function money(cents?: number, currency = 'USD') {
  if (cents == null) return '-'
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: currency.toUpperCase() }).format(cents / 100)
}

function StoreOverview() {
  const { data: store, isLoading } = useStore()
  const listings: StoreListing[] = store?.listings ? Object.values(store.listings) : []

  if (isLoading) {
    return (
      <Container className="mb-6 p-6">
        <div className="h-6 w-48 animate-pulse rounded bg-ui-bg-component" />
        <div className="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-40 animate-pulse rounded-lg bg-ui-bg-component" />
          ))}
        </div>
      </Container>
    )
  }

  if (!store) return null

  return (
    <Container className="mb-6 p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
        <div>
          <Heading level="h3">{store.name}</Heading>
          <Text size="small" className="text-ui-fg-muted">
            {store.domain ?? store.slug}
            {store.currency ? ` · ${store.currency.toUpperCase()}` : ''}
            {` · ${listings.length} listing${listings.length === 1 ? '' : 's'}`}
          </Text>
        </div>
        <Badge color="green">Live</Badge>
      </div>

      {listings.length === 0 ? (
        <Text size="small" className="py-8 text-center text-ui-fg-muted">No listings published yet</Text>
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {listings.slice(0, 8).map((l) => (
            <StoreListingCard key={l.slug} listing={l} storeCurrency={store.currency} />
          ))}
        </div>
      )}
    </Container>
  )
}

function StoreListingCard({ listing, storeCurrency }: { listing: StoreListing; storeCurrency?: string }) {
  const url = listing.headerImage?.url
  return (
    <div className="overflow-hidden rounded-lg border border-ui-border-base bg-ui-bg-subtle">
      <div className="aspect-square w-full bg-ui-bg-component">
        {url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={url} alt={listing.headerImage?.alt ?? listing.name} className="h-full w-full object-cover" loading="lazy" />
        ) : null}
      </div>
      <div className="p-3">
        <Text size="small" weight="plus" className="truncate text-ui-fg-base">{listing.name}</Text>
        <div className="mt-1 flex items-center justify-between">
          <Text size="small" className="text-ui-fg-muted">{money(listing.price, listing.currency ?? storeCurrency ?? 'USD')}</Text>
          <Badge size="2xsmall" color={listing.available ? 'green' : 'grey'}>
            {listing.available ? 'Available' : 'Sold out'}
          </Badge>
        </div>
      </div>
    </div>
  )
}

function RecentOrders() {
  const { data, isLoading } = useCount('order')
  return (
    <Container className="p-6">
      <Heading level="h3">Recent Orders</Heading>
      {isLoading ? (
        <div className="mt-4 space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
          ))}
        </div>
      ) : !data ? (
        <Text size="small" className="mt-4 py-8 text-center text-ui-fg-muted">No orders yet</Text>
      ) : (
        <Text size="small" className="mt-4 py-8 text-center text-ui-fg-muted">
          {data} total orders
        </Text>
      )}
    </Container>
  )
}

function TopProducts() {
  const { data, isLoading } = useCount('product')
  return (
    <Container className="p-6">
      <Heading level="h3">Top Products</Heading>
      {isLoading ? (
        <div className="mt-4 space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-10 animate-pulse rounded bg-ui-bg-component" />
          ))}
        </div>
      ) : !data ? (
        <Text size="small" className="mt-4 py-8 text-center text-ui-fg-muted">No products yet</Text>
      ) : (
        <Text size="small" className="mt-4 py-8 text-center text-ui-fg-muted">
          {data} total products
        </Text>
      )}
    </Container>
  )
}
