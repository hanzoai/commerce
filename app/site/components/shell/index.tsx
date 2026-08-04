"use client"

/**
 * The site chrome — one nav, one footer, every page.
 *
 * Four pages used to carry their own copy of this markup (homepage, learn
 * layout, not-found, global-error), which is four places for the same header to
 * drift. Built on `@hanzo/gui` style props reading the shared
 * `@hanzo/ui/gui-config` scale, so the docs chrome matches the Commerce admin.
 */
import type { ReactNode } from "react"
import NextLink from "next/link"
import { Text, XStack, YStack } from "@hanzo/gui"

const NAV = [
  { label: "Docs", href: "/learn" },
  { label: "Dashboard", href: "https://admin.commerce.hanzo.ai" },
  { label: "GitHub", href: "https://github.com/hanzoai/commerce" },
]

const FOOTER = [
  { label: "GitHub", href: "https://github.com/hanzoai/commerce" },
  { label: "Discord", href: "https://discord.gg/hanzoai" },
  { label: "hanzo.ai", href: "https://hanzo.ai" },
]

/** An anchor for anything that leaves the export, next/link for anything inside. */
export const SiteLink = ({
  href,
  children,
}: {
  href: string
  children: ReactNode
}) =>
  /^[a-z]+:/i.test(href) ? (
    <a href={href}>{children}</a>
  ) : (
    <NextLink href={href}>{children}</NextLink>
  )

export const SiteNav = ({ sticky = false }: { sticky?: boolean }) => (
  <XStack
    render="nav"
    items="center"
    justify="space-between"
    px="$4"
    py="$3"
    borderBottomWidth={1}
    borderColor="$borderColor"
    bg="$background"
    position={sticky ? "sticky" : "relative"}
    t={0}
    z={50}
  >
    <SiteLink href="/">
      <Text fontSize="$6" fontWeight="500" color="$color">
        Hanzo Commerce
      </Text>
    </SiteLink>
    <XStack items="center" gap="$4">
      {NAV.map((item) => (
        <SiteLink key={item.href} href={item.href}>
          <Text fontSize="$3" color="$color11" hoverStyle={{ color: "$color" }}>
            {item.label}
          </Text>
        </SiteLink>
      ))}
    </XStack>
  </XStack>
)

export const SiteFooter = () => (
  <YStack
    render="footer"
    items="center"
    gap="$3"
    px="$4"
    py="$8"
    borderTopWidth={1}
    borderColor="$borderColor"
  >
    <XStack gap="$4">
      {FOOTER.map((item) => (
        <SiteLink key={item.href} href={item.href}>
          <Text fontSize="$3" color="$color11" hoverStyle={{ color: "$color" }}>
            {item.label}
          </Text>
        </SiteLink>
      ))}
    </XStack>
    <Text fontSize="$1" color="$color11">
      &copy; {new Date().getFullYear()} Hanzo Industries. All rights reserved.
    </Text>
  </YStack>
)

/** Nav + content + footer, with the content column carrying the docs measure. */
export const SiteShell = ({
  children,
  stickyNav = false,
  prose = false,
}: {
  children?: ReactNode
  stickyNav?: boolean
  /** Wrap the children in the MDX prose rules (`app/globals.css`). */
  prose?: boolean
}) => (
  <YStack minH="100vh" bg="$background">
    <SiteNav sticky={stickyNav} />
    <YStack flex={1} render="main" className={prose ? "docs-prose" : undefined}>
      {children}
    </YStack>
    <SiteFooter />
  </YStack>
)
