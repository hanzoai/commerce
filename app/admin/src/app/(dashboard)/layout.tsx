'use client'

/**
 * The merchant shell — sidebar + top bar, drawn from the ONE resource catalog.
 *
 * Auth is the only gate: an unauthenticated visitor goes to `/login`, and the IAM
 * bearer is synced into the commerce client so every `/v1` read is org-scoped.
 * The top bar exposes both `OrgProjectSwitcher` (with cross-tenant search for
 * admins across `hanzo`, `hanzo-inc`, `lecce`, `lux`, `zoo`, `pars`, etc.) and
 * `UserMenu` on the same `useActiveOrg()` state.
 */
import { useEffect, type ReactNode } from 'react'
import { usePathname, useRouter } from 'next/navigation'
import { Text, XStack, YStack, useMedia } from '@hanzo/gui'
import { OrgProjectSwitcher, UserMenu, useIam } from '@hanzo/iam/react'
import { HanzoMark, ThemeToggle } from '@hanzo/ui/product'
import {
  Activity,
  ArrowLeftRight,
  Boxes,
  Coins,
  CreditCard,
  FileText,
  Gift,
  LayoutDashboard,
  MapPin,
  Package,
  Percent,
  Receipt,
  Repeat,
  RotateCcw,
  ShoppingBag,
  ShoppingCart,
  Store,
  Users,
  Wallet,
} from '@hanzogui/lucide-icons-2'

import { Palette, openPalette } from '@/components/palette'
import { setAccessToken } from '@/lib/commerce'
import { useActiveOrg } from '@/lib/org'
import { RESOURCES } from '@/lib/resources'

/** Glyph per surface — the catalog stays glyph-free, the chrome draws it. */
const ICONS: Record<string, typeof Package> = {
  overview: LayoutDashboard,
  payments: CreditCard,
  ledger: Wallet,
  orders: ShoppingBag,
  subscriptions: Repeat,
  invoices: FileText,
  payouts: Coins,
  carts: ShoppingCart,
  stores: Store,
  products: Package,
  collections: Boxes,
  discounts: Percent,
  returns: RotateCcw,
  credits: Gift,
  usage: Activity,
  'sales-channels': Store,
  'stock-locations': MapPin,
  subscribers: Users,
  transfers: ArrowLeftRight,
  claims: Receipt,
}

const NAV = [{ slug: 'overview', label: 'Overview' }, ...RESOURCES.map((r) => ({ slug: r.slug, label: r.label }))]

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading, accessToken } = useIam()
  const orgs = useActiveOrg()
  const media = useMedia()
  const router = useRouter()
  const pathname = usePathname() ?? ''

  useEffect(() => setAccessToken(accessToken), [accessToken])

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace('/login')
  }, [isLoading, isAuthenticated, router])

  if (isLoading || !isAuthenticated) return null

  return (
    <XStack minH="100vh" bg="$background">
      <Palette surfaces={NAV} />
      {media.sm ? (
        <YStack width={232} borderRightWidth={1} borderColor="$borderColor" p="$3" gap="$1">
          <XStack items="center" gap="$2" px="$2" py="$3">
            <HanzoMark size={20} />
            <Text fontSize="$4" fontWeight="600">
              Commerce
            </Text>
          </XStack>
          {NAV.map((item) => {
            const href = `/${item.slug}`
            const active = pathname === href || pathname.startsWith(`${href}/`)
            const Icon = ICONS[item.slug] ?? Package
            return (
              <XStack
                key={item.slug}
                items="center"
                gap="$2.5"
                px="$2"
                py="$1.5"
                rounded="$3"
                cursor="pointer"
                bg={active ? '$color3' : undefined}
                hoverStyle={{ bg: '$color2' }}
                onPress={() => router.push(href)}
              >
                <Icon size={15} color={active ? '$color12' : '$color10'} />
                <Text fontSize="$3" color={active ? '$color12' : '$color11'} numberOfLines={1}>
                  {item.label}
                </Text>
              </XStack>
            )
          })}
        </YStack>
      ) : null}

      <YStack flex={1} minW={0}>
        <XStack
          className="hz-topbar"
          items="center"
          justify="space-between"
          gap="$2"
          px="$4"
          py="$2"
          borderBottomWidth={1}
          borderColor="$borderColor"
        >
          <XStack items="center" gap="$2" minW={0}>
            {!media.sm ? (
              <XStack
                items="center"
                gap="$2"
                px="$2.5"
                py="$1.5"
                rounded="$3"
                cursor="pointer"
                borderWidth={1}
                borderColor="$borderColor"
                onPress={openPalette}
                accessibilityLabel="Go to"
                accessibilityRole="button"
              >
                <LayoutDashboard size={15} color="$color11" />
                <Text fontSize="$2" color="$color11">
                  Go to…
                </Text>
              </XStack>
            ) : null}
            <OrgProjectSwitcher
              organizations={orgs.organizations}
              currentOrgId={orgs.currentOrgId}
              switchOrg={orgs.switchOrg}
              findOrgs={orgs.findOrgs}
              alwaysShow
            />
          </XStack>
          <XStack items="center" gap="$2">
            <ThemeToggle />
            <UserMenu orgState={orgs} align="down" />
          </XStack>
        </XStack>
        <YStack flex={1} p="$5" gap="$4">
          {children}
        </YStack>
      </YStack>
    </XStack>
  )
}
