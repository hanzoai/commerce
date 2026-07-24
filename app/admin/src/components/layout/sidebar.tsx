'use client'

import { usePathname } from 'next/navigation'
import Link from 'next/link'
import {
  SquaresPlus,
  AiAssistent,
  ShoppingBag,
  ReceiptPercent,
  Users,
  CurrencyDollar,
  Tag,
  ArchiveBox,
  CogSixTooth,
  XMark,
} from '@hanzo/commerce-icons'
import { Button, Text, clx } from '@hanzo/commerce-ui'
import { useIam, useOrganizations, OrgProjectSwitcher } from '@hanzo/iam/react'
import { HanzoMark } from '@/components/hanzo-mark'
import { AccountMenu } from './account-menu'

const navItems = [
  { label: 'Dashboard', href: '/overview', icon: SquaresPlus },
  { label: 'Models', href: '/models', icon: AiAssistent },
  { label: 'Products', href: '/products', icon: ShoppingBag },
  { label: 'Orders', href: '/orders', icon: ReceiptPercent },
  { label: 'Customers', href: '/customers', icon: Users },
  { label: 'Collections', href: '/collections', icon: Tag },
  { label: 'Inventory', href: '/inventory', icon: ArchiveBox },
  { label: 'Integrations', href: '/integrations', icon: SquaresPlus },
  { label: 'Billing', href: '/billing', icon: CurrencyDollar },
  { label: 'Settings', href: '/settings', icon: CogSixTooth },
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
  const items = navItems.filter((item) => item.href !== '/models' || isHanzoOrg)

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

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        {items.map((item) => {
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
