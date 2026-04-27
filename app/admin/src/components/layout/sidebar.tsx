'use client'

import { usePathname } from 'next/navigation'
import Link from 'next/link'
import {
  SquaresPlus,
  ShoppingBag,
  ReceiptPercent,
  Users,
  CurrencyDollar,
  Tag,
  ArchiveBox,
  CogSixTooth,
} from '@hanzo/commerce-icons'
import { Button, Text, clx } from '@hanzo/commerce-ui'
import { useIam, useOrganizations, OrgProjectSwitcher } from '@hanzo/iam/react'

const navItems = [
  { label: 'Dashboard', href: '/overview', icon: SquaresPlus },
  { label: 'Products', href: '/products', icon: ShoppingBag },
  { label: 'Orders', href: '/orders', icon: ReceiptPercent },
  { label: 'Customers', href: '/customers', icon: Users },
  { label: 'Collections', href: '/collections', icon: Tag },
  { label: 'Inventory', href: '/inventory', icon: ArchiveBox },
  { label: 'Billing', href: '/billing', icon: CurrencyDollar },
  { label: 'Settings', href: '/settings', icon: CogSixTooth },
]

export function Sidebar() {
  const pathname = usePathname()
  const { isAuthenticated, user, login, logout } = useIam()
  const orgState = useOrganizations()

  return (
    <aside className="fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r border-ui-border-base bg-ui-bg-base">
      <div className="flex h-16 items-center gap-3 border-b border-ui-border-base px-6">
        <svg viewBox="0 0 24 24" className="h-8 w-8" xmlns="http://www.w3.org/2000/svg">
          <path d="M3 2 H7 V10 H17 V2 H21 V22 H17 V14 H7 V22 H3 Z" fill="currentColor"/>
        </svg>
        <div>
          <Text size="small" weight="plus" className="text-ui-fg-base">Hanzo Commerce</Text>
          <Text size="xsmall" className="text-ui-fg-muted">Admin Dashboard</Text>
        </div>
      </div>

      {isAuthenticated && (
        <div className="border-b border-ui-border-base px-4 py-3">
          <OrgProjectSwitcher
            {...orgState}
            alwaysShow
            className="w-full"
          />
        </div>
      )}

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        {navItems.map((item) => {
          const isActive = pathname === item.href || pathname.startsWith(item.href + '/')
          return (
            <Link
              key={item.href}
              href={item.href}
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
          <div className="space-y-3">
            <Text size="xsmall" className="truncate px-3 text-ui-fg-muted">
              {user?.email || user?.displayName}
            </Text>
            <Button
              variant="transparent"
              size="small"
              className="w-full justify-start"
              onClick={logout}
            >
              Sign Out
            </Button>
          </div>
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
