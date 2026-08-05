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
import { Text, XStack, YStack, useMedia } from '@hanzo/gui'
import { UserMenu, useIam, useOrganizations } from '@hanzo/iam/react'
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

import { Palette, openPalette } from '@/components/palette'
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
  const orgs = useOrganizations()
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
      {/* 232px OF A 390px VIEWPORT IS NOT A SIDEBAR, IT IS THE PAGE. Measured:
          it took 59.5% of the width, left 158px for content, and pushed the
          document 66px wider than the screen — and because it is in flow rather
          than fixed, scrolling right to read the content clipped the nav labels
          it was crowding them with ("Overview" -> "verview"). You lost the nav to
          see content and the content to see the nav.

          It steps aside below the breakpoint rather than growing a drawer,
          because the drawer already exists: the palette lists every surface in
          the same catalog this sidebar renders, so there is one navigation, not
          two that can disagree. What it lacked was a door reachable without a
          keyboard — that is the button in the top bar. */}
      {/* `sm` is minWidth:640 in the v5 config — TRUE above the breakpoint, so
          390px is false and 1440px is true. (`gtSm` is the v4 vocabulary and is
          not declared on v5's UseMediaState; the typechecker caught it.) */}
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
          justify="flex-end"
          gap="$2"
          px="$4"
          py="$2"
          borderBottomWidth={1}
          borderColor="$borderColor"
        >
          {/* ONE CONTROL: who you are AND which store you are editing.
              `UserMenu` is published by @hanzo/iam/react and takes the org state
              directly, so identity and the switcher are the same menu on every
              Hanzo surface. This was three things — a bare `currentOrgId`
              string that could not be operated, an empty native <select> in the
              sidebar, and no way to sign out at all — and then briefly two, when
              I paired the switcher with an account menu written here. A second
              account menu is a second answer to "who is signed in"; deleting it
              is the fix, not maintaining it.

              It also needs the CURRENT @hanzo/iam: on the 0.13 line the org hook
              synthesised at most ONE org from the `owner` claim, and the switcher
              hides itself at one org, so it could never list anything. 0.21 reads
              the signed `orgs` membership set — home first — which is the same
              set cloud enforces when it honours an X-Org-Id. */}
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
          <ThemeToggle />
          <UserMenu orgState={orgs} align="down" />
        </XStack>
        <YStack flex={1} p="$5" gap="$4">
          {children}
        </YStack>
      </YStack>
    </XStack>
  )
}
