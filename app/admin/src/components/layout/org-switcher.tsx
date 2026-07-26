'use client'

import { Buildings, ChevronUpDown, CheckMini } from '@hanzo/commerce-icons'
import { Avatar, DropdownMenu, Text, clx } from '@hanzo/commerce-ui'
import { useOrganizations } from '@hanzo/iam/react'

function orgLabel(org: { displayName?: string; name?: string } | null): string {
  return org?.displayName || org?.name || 'Organization'
}

/**
 * Canonical organization switcher — the sole tenant selector.
 *
 * Replaces the bespoke `<OrgProjectSwitcher>` native `<select>` with the Hanzo
 * dropdown surface (commerce-ui DropdownMenu + Avatar), wired to the SAME
 * `@hanzo/iam` `useOrganizations` the rest of the app reads. Single org ⇒ a
 * static, non-interactive label (nothing to switch to).
 */
export function OrgSwitcher({ className }: { className?: string }) {
  const { organizations, currentOrg, currentOrgId, switchOrg, isLoading } =
    useOrganizations()

  const label = orgLabel(currentOrg)
  const initial = label[0]?.toUpperCase() || '?'

  const badge = (
    <span className="grid grid-cols-[24px_1fr_16px] items-center gap-2">
      <Avatar size="xsmall" fallback={initial} />
      <span className="min-w-0 text-left">
        <Text size="small" weight="plus" leading="compact" className="block truncate text-ui-fg-base">
          {label}
        </Text>
      </span>
      {organizations.length > 1 && (
        <ChevronUpDown className="h-4 w-4 text-ui-fg-muted" />
      )}
    </span>
  )

  const shell =
    'flex w-full items-center rounded-md border border-ui-border-base bg-ui-bg-field px-2 py-1.5 text-left'

  // Nothing to switch to: render a static, honest label (not a dead button).
  if (organizations.length <= 1) {
    return (
      <div className={clx(shell, className)} aria-label="Current organization">
        {badge}
      </div>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenu.Trigger
        disabled={isLoading}
        aria-label="Switch organization"
        className={clx(
          shell,
          'outline-none transition-colors hover:bg-ui-bg-field-hover',
          'focus-visible:shadow-borders-focus data-[state=open]:bg-ui-bg-field-hover',
          className,
        )}
      >
        {badge}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content
        align="start"
        className="min-w-[var(--radix-dropdown-menu-trigger-width)]"
      >
        <DropdownMenu.Label className="flex items-center gap-2">
          <Buildings className="h-4 w-4 text-ui-fg-muted" />
          Organizations
        </DropdownMenu.Label>
        <DropdownMenu.Separator />
        {organizations.map((org) => {
          const active = org.name === currentOrgId
          return (
            <DropdownMenu.Item
              key={org.name}
              onClick={() => switchOrg(org.name)}
              className="grid grid-cols-[1fr_16px] items-center gap-2"
            >
              <span className="min-w-0 truncate">{orgLabel(org)}</span>
              {active && <CheckMini className="h-4 w-4 text-ui-fg-interactive" />}
            </DropdownMenu.Item>
          )
        })}
      </DropdownMenu.Content>
    </DropdownMenu>
  )
}
