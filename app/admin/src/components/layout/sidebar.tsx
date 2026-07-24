'use client'

import { usePathname } from 'next/navigation'
import Link from 'next/link'
import {
  SquaresPlus,
  AiAssistent,
  ShoppingBag,
  ShoppingCart,
  ReceiptPercent,
  Sparkles,
  Users,
  UserGroup,
  IdBadge,
  CurrencyDollar,
  ChartBar,
  Tag,
  TablePen,
  GiftCards,
  ArchiveBox,
  Map,
  BuildingTax,
  Channels,
  Key,
  Plug,
  CogSixTooth,
  XMark,
} from '@hanzo/commerce-icons'
import { Button, Text, clx } from '@hanzo/commerce-ui'
import { useIam, useOrganizations, OrgProjectSwitcher } from '@hanzo/iam/react'
import { HanzoMark } from '@/components/hanzo-mark'
import { AccountMenu } from './account-menu'

type NavItem = { label: string; href: string; icon: typeof SquaresPlus }
type NavGroup = { heading?: string; items: NavItem[] }

// Grouped navigation. Each group is one concern; the flat pre-group order
// (overview → models → catalog → sales → settings) is preserved within.
const navGroups: NavGroup[] = [
  {
    items: [
      { label: 'Dashboard', href: '/overview', icon: SquaresPlus },
      { label: 'Models', href: '/models', icon: AiAssistent },
    ],
  },
  {
    heading: 'Catalog',
    items: [
      { label: 'Products', href: '/products', icon: ShoppingBag },
      { label: 'Collections', href: '/collections', icon: Tag },
      { label: 'Inventory', href: '/inventory', icon: ArchiveBox },
      { label: 'Price Lists', href: '/price-lists', icon: TablePen },
      { label: 'Gift Cards', href: '/gift-cards', icon: GiftCards },
    ],
  },
  {
    heading: 'Sales',
    items: [
      { label: 'Orders', href: '/orders', icon: ShoppingCart },
      { label: 'Customers', href: '/customers', icon: Users },
      { label: 'Customer Groups', href: '/customer-groups', icon: UserGroup },
    ],
  },
  {
    heading: 'Marketing',
    items: [
      { label: 'Discounts', href: '/discounts', icon: ReceiptPercent },
      { label: 'Promotions', href: '/promotions', icon: Sparkles },
    ],
  },
  {
    heading: 'Configuration',
    items: [
      { label: 'Regions', href: '/regions', icon: Map },
      { label: 'Tax', href: '/tax-regions', icon: BuildingTax },
      { label: 'Sales Channels', href: '/sales-channels', icon: Channels },
    ],
  },
  {
    heading: 'Developer',
    items: [
      { label: 'API Keys', href: '/api-keys', icon: Key },
      { label: 'Integrations', href: '/integrations', icon: Plug },
    ],
  },
  {
    heading: 'Account',
    items: [
      { label: 'Usage', href: '/usage', icon: ChartBar },
      { label: 'Billing', href: '/billing', icon: CurrencyDollar },
      { label: 'Team', href: '/team', icon: IdBadge },
      { label: 'Settings', href: '/settings', icon: CogSixTooth },
    ],
  },
]

export function Sidebar({
  open = false,
  onClose = () => undefined,
}: {
  open?: boolean
  onClose?: () => void
}) {
  const pathname = usePathname()
  const { isAuthenticated, login } = useIam()
  const orgState = useOrganizations()

  // Models is a Hanzo-internal surface (the AI model catalog), not a tenant
  // commerce feature — show it only when the active org is one of Hanzo's own.
  const orgName = (orgState.currentOrg?.name || '').toLowerCase()
  const isHanzoOrg = orgName === 'hanzo' || orgName === 'admin'
  const groups = navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => item.href !== '/models' || isHanzoOrg),
    }))
    .filter((group) => group.items.length > 0)

  return (
    <aside
      aria-label="Main navigation"
      className={clx(
        'fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-ui-border-base bg-ui-bg-base transition-transform lg:translate-x-0',
        open ? 'translate-x-0' : '-translate-x-full',
      )}
    >
      <div className="flex h-16 items-center border-b border-ui-border-base px-4">
        <Link href="/overview" onClick={onClose} className="flex min-w-0 flex-1 items-center gap-3 rounded-md px-2 py-1">
          <HanzoMark className="h-8 w-8 shrink-0 text-ui-fg-base" />
          <Text size="small" weight="plus" className="truncate text-ui-fg-base">
            Hanzo Commerce
          </Text>
        </Link>
        <button
          type="button"
          aria-label="Close navigation"
          onClick={onClose}
          className="rounded-md p-2 text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base lg:hidden"
        >
          <XMark className="h-5 w-5" />
        </button>
      </div>

      {isAuthenticated && (
        <div className="border-b border-ui-border-base px-4 py-3">
          <Text size="xsmall" weight="plus" className="mb-1.5 block text-ui-fg-muted">
            Organization
          </Text>
          <OrgProjectSwitcher
            {...orgState}
            alwaysShow
            className="w-full [&>select]:h-9 [&>select]:w-full [&>select]:min-w-0 [&>select]:border-ui-border-base [&>select]:bg-ui-bg-field [&>select]:text-ui-fg-base"
          />
        </div>
      )}

      <nav className="flex-1 space-y-4 overflow-y-auto px-3 py-4">
        {groups.map((group, i) => (
          <div key={group.heading ?? `group-${i}`} className="space-y-1">
            {group.heading && (
              <Text
                size="xsmall"
                weight="plus"
                className="px-3 pb-1 pt-2 uppercase tracking-wide text-ui-fg-muted"
              >
                {group.heading}
              </Text>
            )}
            {group.items.map((item) => {
              const isActive = pathname === item.href || pathname.startsWith(item.href + '/')
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={onClose}
                  className={clx(
                    'flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
                    isActive
                      ? 'bg-ui-bg-component text-ui-fg-base'
                      : 'text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base'
                  )}
                >
                  <item.icon className="h-5 w-5" />
                  {item.label}
                </Link>
              )
            })}
          </div>
        ))}
      </nav>

      <div className="border-t border-ui-border-base p-4">
        {isAuthenticated ? (
          <AccountMenu />
        ) : (
          <Button
            variant="transparent"
            size="small"
            className="w-full justify-start"
            onClick={() => login()}
          >
            Sign In
          </Button>
        )}
      </div>
    </aside>
  )
}
