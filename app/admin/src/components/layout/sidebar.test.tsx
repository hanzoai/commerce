import React from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

vi.mock('next/navigation', () => ({
  usePathname: () => '/billing',
}))

vi.mock('@hanzo/iam/react', () => ({
  useIam: () => ({
    isAuthenticated: true,
    login: vi.fn(),
  }),
  useOrganizations: () => ({
    currentOrg: { name: 'hanzo', displayName: 'Hanzo' },
    currentOrgId: 'hanzo',
    organizations: [{ name: 'hanzo', displayName: 'Hanzo' }],
    projects: [],
    currentProjectId: null,
    switchOrg: vi.fn(),
    switchProject: vi.fn(),
  }),
  OrgProjectSwitcher: () => 'organization-switcher',
}))

vi.mock('@/lib/api/hooks', () => ({
  useStore: () => ({ data: { id: 'store_1', name: 'Hanzo Agency' } }),
}))

vi.mock('./store-menu', () => ({
  StoreMenu: () => 'store-switcher',
}))

vi.mock('./account-menu', () => ({
  AccountMenu: () => 'account-menu',
}))

import { Sidebar } from './sidebar'

describe('Sidebar', () => {
  it('keeps product, organization, store, and person identity separate', () => {
    const html = renderToStaticMarkup(<Sidebar />)

    expect(html).toContain('Hanzo Commerce')
    expect(html).not.toContain('Hanzo Agency')
    expect(html).toContain('Organization')
    expect(html).toContain('organization-switcher')
    expect(html).toContain('account-menu')
  })
})
