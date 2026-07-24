'use client'

import { useEffect, useRef, useState } from 'react'
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
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [])

  return (
    <div ref={root} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className="flex items-center gap-2 rounded-md px-2 py-1.5 text-left hover:bg-ui-bg-component"
      >
        <span className="flex h-8 w-8 items-center justify-center rounded-full bg-ui-bg-component text-sm font-medium text-ui-fg-base">
          {initial}
        </span>
        <Text size="small" className="hidden max-w-44 truncate text-ui-fg-muted sm:block">
          {name}
        </Text>
        <span aria-hidden className="text-xs text-ui-fg-muted">⌄</span>
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 mt-2 w-64 rounded-lg border border-ui-border-base bg-ui-bg-subtle p-2 shadow-elevation-flyout"
        >
          <div className="border-b border-ui-border-base px-2 pb-2">
            <Text size="xsmall" className="truncate text-ui-fg-muted">{user?.email}</Text>
          </div>
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
            className="w-full rounded-md px-2 py-2 text-left text-sm text-ui-fg-muted hover:bg-ui-bg-component hover:text-ui-fg-base"
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  )
}
