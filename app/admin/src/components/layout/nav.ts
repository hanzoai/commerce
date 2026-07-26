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
} from '@hanzo/commerce-icons'

export type NavItem = { label: string; href: string; icon: typeof SquaresPlus }
export type NavGroup = { heading?: string; items: NavItem[] }

// The ONE navigation definition. The sidebar renders it as grouped links; the
// command palette (⌘K) searches the same items — one source, never two lists.
export const navGroups: NavGroup[] = [
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

// Models is a Hanzo-internal surface (the AI model catalog), not a tenant
// commerce feature — visible only when the active org is one of Hanzo's own.
export function isHanzoOrgName(name: string | undefined | null): boolean {
  const n = (name || '').toLowerCase()
  return n === 'hanzo' || n === 'admin'
}

// The nav groups the active org may see, with empty groups dropped.
export function visibleGroups(isHanzoOrg: boolean): NavGroup[] {
  return navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => item.href !== '/models' || isHanzoOrg),
    }))
    .filter((group) => group.items.length > 0)
}
