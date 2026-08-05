'use client'

/**
 * The merchant shell — sidebar + top bar, drawn from the ONE resource catalog.
 *
 * Auth is the only gate: an unauthenticated visitor goes to `/login`, and the IAM
 * bearer is synced into the commerce client so every `/v1` read is org-scoped.
 * Everything visual is `@hanzo/ui/product` — the same components the console
 * renders, so the two admins cannot drift.
 */
import { useEffect, type ReactNode } from 'react'
import { usePathname, useRouter } from 'next/navigation'
import { Text, XStack, YStack } from '@hanzo/gui'
import { useIam, useOrganizations } from '@hanzo/iam/react'
import { HanzoMark, ThemeToggle } from '@hanzo/ui/product'
import {
  Boxes,
  CreditCard,
  FileText,
  Gift,
  KeyRound,
  LayoutDashboard,
  MapPin,
  Package,
  Percent,
  Receipt,
  Settings,
  ShoppingBag,
  Store,
  Tags,
  Users,
  Warehouse,
} from '@hanzogui/lucide-icons-2'

import { setAccessToken } from '@/lib/commerce'
import { RESOURCES } from '@/lib/resources'

/** Glyph per surface — the catalog stays glyph-free, the chrome draws it. */
const ICONS: Record<string, typeof Package> = {
  overview: LayoutDashboard,
  products: Package,
  collections: Boxes,
  categories: Tags,
  types: FileText,
  orders: ShoppingBag,
  claims: Receipt,
  customers: Users,
  'customer-groups': Users,
  'inventory-items': Warehouse,
  'stock-locations': MapPin,
  reservations: Warehouse,
  promotions: Percent,
  'price-lists': Gift,
  regions: MapPin,
  'tax-regions': Percent,
  currencies: CreditCard,
  'sales-channels': Store,
  'api-keys': KeyRound,
  roles: Settings,
}

const NAV = [{ slug: 'overview', label: 'Overview' }, ...RESOURCES.map((r) => ({ slug: r.slug, label: r.label }))]

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading, accessToken } = useIam()
  const { currentOrgId } = useOrganizations()
  const router = useRouter()
  const pathname = usePathname() ?? ''

  useEffect(() => setAccessToken(accessToken), [accessToken])

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace('/login')
  }, [isLoading, isAuthenticated, router])

  if (isLoading || !isAuthenticated) return null

  return (
    <XStack minH="100vh" bg="$background">
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

      <YStack flex={1} minW={0}>
        <XStack
          className="hz-topbar"
          items="center"
          justify="flex-end"
          gap="$2"
          px="$4"
          py="$2"
          borderBottomWidth={1}
          borderColor="$borderColor"
        >
          {currentOrgId ? (
            <Text fontSize="$2" color="$color10" numberOfLines={1}>
              {currentOrgId}
            </Text>
          ) : null}
          <ThemeToggle />
        </XStack>
        <YStack flex={1} p="$5" gap="$4">
          {children}
        </YStack>
      </YStack>
    </XStack>
  )
}
