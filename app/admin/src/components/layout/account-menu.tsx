'use client'

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import {
  BookOpen,
  EllipsisHorizontal,
  OpenRectArrowOut,
  User,
  Users,
} from '@hanzo/commerce-icons'
import { Text, clx } from '@hanzo/commerce-ui'
import { useIam } from '@hanzo/iam/react'
import { useTheme } from '@/providers/theme-provider'
import type { ThemeOption } from '@/providers/theme-provider/theme-context'

const themes: ThemeOption[] = ['dark', 'light', 'system']

export function AccountMenu() {
  const { user, logout } = useIam()
  const { theme, setTheme } = useTheme()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const name = user?.displayName || user?.email || 'Account'
  const initial = name[0]?.toUpperCase() || '?'

  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('mousedown', close)
      document.removeEventListener('keydown', escape)
    }
  }, [])

  return (
    <div ref={root} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className="grid w-full grid-cols-[32px_1fr_16px] items-center gap-2 rounded-md px-2 py-1.5 text-left hover:bg-ui-bg-component"
      >
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-ui-bg-component text-sm font-medium text-ui-fg-base">
          {initial}
        </span>
        <span className="min-w-0">
          <Text size="small" weight="plus" className="block truncate text-ui-fg-base">
            {name}
          </Text>
          <Text size="xsmall" className="block truncate text-ui-fg-muted">
            {user?.email}
          </Text>
        </span>
        <EllipsisHorizontal className="h-4 w-4 text-ui-fg-muted" />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute bottom-full left-0 mb-2 w-full rounded-lg border border-ui-border-base bg-ui-bg-subtle p-2 shadow-elevation-flyout"
        >
          <MenuLink href="https://console.hanzo.ai/profile" icon={User}>
            Profile & security
          </MenuLink>
          <MenuLink href="/team" icon={Users} internal onNavigate={() => setOpen(false)}>
            Organization & team
          </MenuLink>
          <MenuLink href="https://docs.hanzo.ai/commerce" icon={BookOpen} external>
            Documentation
          </MenuLink>
          <div className="my-2 border-t border-ui-border-base" />
          <div className="px-2 py-3">
            <Text size="xsmall" weight="plus" className="mb-2 text-ui-fg-muted">Theme</Text>
            <div className="grid grid-cols-3 gap-1 rounded-md bg-ui-bg-base p-1">
              {themes.map((value) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setTheme(value)}
                  className={clx(
                    'rounded px-2 py-1.5 text-xs capitalize text-ui-fg-muted',
                    theme === value && 'bg-ui-bg-component text-ui-fg-base',
                  )}
                >
                  {value}
                </button>
              ))}
            </div>
          </div>
          <button
            type="button"
            role="menuitem"
            onClick={() => logout()}
            className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base"
          >
            <OpenRectArrowOut className="h-4 w-4 text-ui-fg-subtle" />
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}

function MenuLink({
  href,
  icon: Icon,
  external = false,
  internal = false,
  onNavigate,
  children,
}: {
  href: string
  icon: typeof User
  external?: boolean
  /** Route within the admin SPA — rendered as a client-side <Link>. */
  internal?: boolean
  onNavigate?: () => void
  children: React.ReactNode
}) {
  const className =
    'flex items-center gap-2 rounded-md px-2 py-2 text-sm text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base'

  if (internal) {
    return (
      <Link href={href} role="menuitem" className={className} onClick={onNavigate}>
        <Icon className="h-4 w-4 text-ui-fg-subtle" />
        {children}
      </Link>
    )
  }

  return (
    <a
      href={href}
      role="menuitem"
      target={external ? '_blank' : undefined}
      rel={external ? 'noreferrer' : undefined}
      className={className}
    >
      <Icon className="h-4 w-4 text-ui-fg-subtle" />
      {children}
    </a>
  )
}
