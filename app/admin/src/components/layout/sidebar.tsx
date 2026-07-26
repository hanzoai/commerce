'use client'

import { usePathname } from 'next/navigation'
import Link from 'next/link'
import { XMark } from '@hanzo/commerce-icons'
import { Button, Text, clx } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { HanzoMark } from '@/components/hanzo-mark'
import { AccountMenu } from './account-menu'
import { OrgSwitcher } from './org-switcher'
import { visibleGroups, isHanzoOrgName } from './nav'

export function Sidebar({
  open = false,
  onClose = () => undefined,
}: {
  open?: boolean
  onClose?: () => void
}) {
  const pathname = usePathname()
  const { isAuthenticated, login } = useIam()
  const { currentOrg } = useOrganizations()

  // Models is a Hanzo-internal surface (the AI model catalog), not a tenant
  // commerce feature — visible only when the active org is one of Hanzo's own.
  const groups = visibleGroups(isHanzoOrgName(currentOrg?.name))

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
          <OrgSwitcher />
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
