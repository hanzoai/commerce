'use client'

// The settings HUB: an index of cards linking to each configuration sub-section.
// Store details and notifications are owned here (real sub-pages under settings/);
// regions, tax, sales channels, developer keys, team, and billing are owned by
// their top-level domain pages, so the hub LINKS to them rather than duplicating.
import Link from 'next/link'
import { Container, Heading, Text } from '@hanzo/commerce-ui'
import {
  BuildingStorefront,
  BellAlert,
  Map,
  BuildingTax,
  Channels,
  Key,
  Plug,
  IdBadge,
  CurrencyDollar,
  ChevronRight,
} from '@hanzo/commerce-icons'
import { PageHeader } from '@/components/common/page-header'

type Item = { label: string; description: string; href: string; icon: typeof Key }
type Section = { heading: string; items: Item[] }

const sections: Section[] = [
  {
    heading: 'General',
    items: [
      { label: 'Store details', description: 'Name, currency, domain, and branding', href: '/settings/store', icon: BuildingStorefront },
      { label: 'Notifications', description: 'Choose which events email you', href: '/settings/notifications', icon: BellAlert },
    ],
  },
  {
    heading: 'Regions & Tax',
    items: [
      { label: 'Regions', description: 'Countries, currencies, and fulfillment zones', href: '/regions', icon: Map },
      { label: 'Tax', description: 'Tax regions and rates', href: '/tax-regions', icon: BuildingTax },
    ],
  },
  {
    heading: 'Selling',
    items: [
      { label: 'Sales channels', description: 'Storefronts and points of sale', href: '/sales-channels', icon: Channels },
    ],
  },
  {
    heading: 'Developer',
    items: [
      { label: 'API keys', description: 'Publishable and secret keys', href: '/api-keys', icon: Key },
      { label: 'Integrations', description: 'Payments, fulfillment, and marketing providers', href: '/integrations', icon: Plug },
    ],
  },
  {
    heading: 'Account',
    items: [
      { label: 'Team & access', description: 'Members, roles, and invitations', href: '/team', icon: IdBadge },
      { label: 'Billing', description: 'Subscription, invoices, and payment method', href: '/billing', icon: CurrencyDollar },
    ],
  },
]

export default function SettingsPage() {
  return (
    <div>
      <PageHeader title="Settings" description="Configure your store, developer access, and account" />
      <div className="space-y-8 p-8">
        {sections.map((section) => (
          <section key={section.heading}>
            <Heading level="h3" className="mb-3 text-ui-fg-subtle">
              {section.heading}
            </Heading>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {section.items.map((item) => (
                <Link key={item.href} href={item.href} className="block">
                  <Container className="flex items-center gap-4 p-5 transition-colors hover:border-ui-border-strong">
                    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-ui-bg-component text-ui-fg-base">
                      <item.icon className="h-5 w-5" />
                    </span>
                    <div className="min-w-0 flex-1">
                      <Text size="small" weight="plus" className="text-ui-fg-base">
                        {item.label}
                      </Text>
                      <Text size="xsmall" className="mt-0.5 text-ui-fg-muted">
                        {item.description}
                      </Text>
                    </div>
                    <ChevronRight className="h-5 w-5 shrink-0 text-ui-fg-muted" />
                  </Container>
                </Link>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  )
}
