'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Command } from 'cmdk'
import {
  MagnifyingGlass,
  Buildings,
  Sun,
  Moon,
  ComputerDesktop,
  OpenRectArrowOut,
  CheckMini,
} from '@hanzo/commerce-icons'
import { Kbd } from '@hanzo/commerce-ui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { visibleGroups, isHanzoOrgName } from './nav'
import { useTheme } from '@/providers/theme-provider'
import type { ThemeOption } from '@/providers/theme-provider/theme-context'

const themeIcons: Record<ThemeOption, typeof Sun> = {
  light: Sun,
  dark: Moon,
  system: ComputerDesktop,
}

/**
 * Canonical ⌘K command palette — search over the whole shell.
 *
 * ONE surface for: jumping to any nav destination (the same `visibleGroups`
 * the sidebar renders — Models-gated identically), switching organization
 * (`@hanzo/iam` `useOrganizations`), theme, and sign-out. `cmdk` does the
 * fuzzy match; styling is pure commerce-ui tokens so it matches the dark shell.
 */
export function CommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const router = useRouter()
  const { logout } = useIam()
  const { organizations, currentOrgId, switchOrg, currentOrg } = useOrganizations()
  const { theme, setTheme } = useTheme()

  // ⌘K / Ctrl+K toggles the palette from anywhere in the shell.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        onOpenChange(!open)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onOpenChange])

  const groups = visibleGroups(isHanzoOrgName(currentOrg?.name))

  const run = (action: () => void) => {
    onOpenChange(false)
    action()
  }

  return (
    <Command.Dialog
      open={open}
      onOpenChange={onOpenChange}
      label="Command palette"
      shouldFilter
      className="fixed inset-0 z-[90]"
      overlayClassName="fixed inset-0 z-[90] bg-ui-bg-overlay"
      contentClassName="fixed left-1/2 top-[15%] z-[91] w-[min(640px,92vw)] -translate-x-1/2 overflow-hidden rounded-xl border border-ui-border-base bg-ui-bg-subtle shadow-elevation-modal"
    >
      <div className="flex items-center gap-2 border-b border-ui-border-base px-4">
        <MagnifyingGlass className="h-4 w-4 shrink-0 text-ui-fg-muted" />
        <Command.Input
          autoFocus
          placeholder="Search pages, organizations, actions…"
          className="h-12 w-full bg-transparent text-sm text-ui-fg-base outline-none placeholder:text-ui-fg-muted"
        />
        <Kbd>esc</Kbd>
      </div>

      <Command.List className="max-h-[min(420px,60vh)] overflow-y-auto p-2">
        <Command.Empty className="px-3 py-6 text-center text-sm text-ui-fg-muted">
          No results found.
        </Command.Empty>

        {groups.map((group, i) => (
          <Command.Group
            key={group.heading ?? `group-${i}`}
            heading={group.heading ?? 'Overview'}
            className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-ui-fg-muted"
          >
            {group.items.map((item) => (
              <Command.Item
                key={item.href}
                value={`${item.label} ${group.heading ?? ''}`}
                onSelect={() => run(() => router.push(item.href))}
                className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-sm text-ui-fg-base aria-selected:bg-ui-bg-component"
              >
                <item.icon className="h-4 w-4 text-ui-fg-muted" />
                {item.label}
              </Command.Item>
            ))}
          </Command.Group>
        ))}

        {organizations.length > 1 && (
          <Command.Group
            heading="Switch organization"
            className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-ui-fg-muted"
          >
            {organizations.map((org) => (
              <Command.Item
                key={org.name}
                value={`org ${org.displayName || org.name}`}
                onSelect={() => run(() => switchOrg(org.name))}
                className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-sm text-ui-fg-base aria-selected:bg-ui-bg-component"
              >
                <Buildings className="h-4 w-4 text-ui-fg-muted" />
                <span className="min-w-0 flex-1 truncate">{org.displayName || org.name}</span>
                {org.name === currentOrgId && (
                  <CheckMini className="h-4 w-4 text-ui-fg-interactive" />
                )}
              </Command.Item>
            ))}
          </Command.Group>
        )}

        <Command.Group
          heading="Preferences"
          className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wide [&_[cmdk-group-heading]]:text-ui-fg-muted"
        >
          {(['dark', 'light', 'system'] as ThemeOption[]).map((value) => {
            const Icon = themeIcons[value]
            return (
              <Command.Item
                key={value}
                value={`theme ${value}`}
                onSelect={() => run(() => setTheme(value))}
                className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-sm capitalize text-ui-fg-base aria-selected:bg-ui-bg-component"
              >
                <Icon className="h-4 w-4 text-ui-fg-muted" />
                <span className="flex-1">{value} theme</span>
                {theme === value && <CheckMini className="h-4 w-4 text-ui-fg-interactive" />}
              </Command.Item>
            )
          })}
          <Command.Item
            value="sign out logout"
            onSelect={() => run(() => logout())}
            className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-sm text-ui-fg-base aria-selected:bg-ui-bg-component"
          >
            <OpenRectArrowOut className="h-4 w-4 text-ui-fg-muted" />
            Sign out
          </Command.Item>
        </Command.Group>
      </Command.List>
    </Command.Dialog>
  )
}
