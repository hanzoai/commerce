"use client"

/**
 * The homepage, as five sections built from three shapes.
 *
 * What this replaces: eleven forked marketing components (Bloom, Edges, two
 * flavours of DottedSeparator, SectionsSeparator, two 300-line inline-SVG icon
 * files, a newsletter code mock, …) that between them carried ~230 Tailwind
 * class strings and a private grey scale. The content moved to `content.ts`;
 * the layout is `@hanzo/gui` style props on the shared `@hanzo/ui/gui-config`
 * scale. Decoration that said nothing did not survive the move.
 *
 * Gui pitfalls this file is written against (each fails silently — an
 * unrecognised prop is dropped, not rejected): `render` names the element, not
 * `tag`; `$md`/`$lg` are mobile-first min-widths, not `$gtMd`; a bare-number
 * `lineHeight` is pixels, so a ratio is a string on `style`; `display: grid`
 * rides `style` because Gui's display union is React-Native-derived.
 */
import { useState, type CSSProperties, type ReactNode } from "react"
import { Text, View, XStack, YStack } from "@hanzo/gui"

import { SiteLink } from "@/components/shell"
import {
  CODE_SAMPLES,
  LINK_GROUPS,
  MODULE_GROUPS,
  RECIPES,
} from "./content"

/** CSS grid — Gui's `display` union is React-Native-derived and stops at flex. */
const GRID: CSSProperties = { display: "grid" }
/** A ratio, not a length: Gui appends `px` to a bare number, prop or style alike. */
const DISPLAY_LEADING: CSSProperties = { lineHeight: "1.15" }

/**
 * The mono face.
 *
 * NOT a Gui token: the shared scale (`@hanzo/ui/gui-config`) declares exactly
 * two families, `body` and `heading`, so `fontFamily="$mono"` names nothing.
 * Gui drops an unrecognised value silently, so this would have shipped as the
 * body face with no error anywhere. It rides `style`, reading the variable
 * `app/fonts.ts` binds — the same one `.docs-prose code` uses, so prose code and
 * component code are one face.
 */
const MONO: CSSProperties = {
  fontFamily: "var(--font-geist-mono), ui-monospace, monospace",
}

/**
 * A code block scrolls sideways rather than reflowing.
 *
 * `overflow` is a Gui style prop but its value union is React-Native's
 * (visible/hidden/scroll) — `auto` is not in it, and the axis-specific
 * `overflowX` does not exist there at all. Both ride `style`.
 */
const CODE_SCROLL: CSSProperties = { overflowX: "auto", whiteSpace: "pre" }

const SECTION = {
  width: "100%",
  borderBottomWidth: 1,
  borderColor: "$borderColor",
} as const

/** The pill + link pair that opens every section. */
const Eyebrow = ({
  label,
  linkText,
  href,
}: {
  label: string
  linkText: string
  href: string
}) => (
  <XStack items="center" gap="$2">
    <Text
      px="$2"
      py="$0.5"
      rounded="$3"
      borderWidth={1}
      borderColor="$borderColor"
      fontSize="$1"
      color="$color11"
    >
      {label}
    </Text>
    <SiteLink href={href}>
      <Text fontSize="$2" color="$color" hoverStyle={{ opacity: 0.7 }}>
        {linkText}
      </Text>
    </SiteLink>
  </XStack>
)

/** A responsive n-up grid; one column on a phone. */
const Grid = ({
  columns,
  children,
}: {
  columns: number
  children: ReactNode
}) => (
  <View
    style={GRID}
    gridTemplateColumns="1fr"
    $sm={{ gridTemplateColumns: "repeat(2, minmax(0, 1fr))" }}
    $lg={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
  >
    {children}
  </View>
)

/**
 * One tile of a `Grid`.
 *
 * Only the right and bottom edges are drawn: a full border on every cell doubles
 * up on every shared edge, and the negative margin that used to hide that made
 * each cell 4px wider than its column — a 1px horizontal overflow on a phone,
 * invisible only because the body clips it. The section owns its outer bottom
 * edge (`SECTION`) and the page frame owns the sides.
 */
const Cell = ({ children }: { children: ReactNode }) => (
  <YStack
    p="$5"
    gap="$3"
    borderRightWidth={1}
    borderBottomWidth={1}
    borderColor="$borderColor"
    hoverStyle={{ bg: "$background04" }}
  >
    {children}
  </YStack>
)

/* ── hero ─────────────────────────────────────────────────────────────────── */

const CTAS = [
  { label: "Get Started", href: "/learn", primary: true },
  { label: "Dashboard", href: "https://admin.commerce.hanzo.ai" },
  { label: "Storefront", href: "https://commerce.hanzo.ai/store" },
]

const Hero = () => (
  <YStack {...SECTION} items="center" gap="$4" px="$4" py="$11">
    <Eyebrow
      label="Hanzo Commerce Documentation"
      linkText="Introduction"
      href="/learn"
    />
    <YStack items="center" gap="$3" maxW={672}>
      <Text
        render="h1"
        text="center"
        fontSize="$11"
        $md={{ fontSize: "$12" }}
        fontWeight="500"
        color="$color"
        style={DISPLAY_LEADING}
      >
        AI-powered commerce infrastructure for modern businesses.
      </Text>
      <Text render="p" text="center" fontSize="$6" color="$color11">
        Build, customize, and scale your commerce platform with built-in modules
        for products, orders, payments, and more.
      </Text>
    </YStack>
    <XStack gap="$3" mt="$3" flexWrap="wrap" justify="center">
      {CTAS.map((cta) => (
        <SiteLink key={cta.href} href={cta.href}>
          <Text
            px="$5"
            py="$2.5"
            rounded="$3"
            fontSize="$3"
            fontWeight="500"
            borderWidth={1}
            borderColor={cta.primary ? "$color" : "$borderColor"}
            bg={cta.primary ? "$color" : "transparent"}
            color={cta.primary ? "$background" : "$color"}
            hoverStyle={{ opacity: 0.85 }}
          >
            {cta.label}
          </Text>
        </SiteLink>
      ))}
    </XStack>
  </YStack>
)

/* ── link groups ──────────────────────────────────────────────────────────── */

const Links = () => (
  <View {...SECTION}>
    <Grid columns={3}>
      {LINK_GROUPS.map((group) => (
        <Cell key={group.tag}>
          <Text
            fontSize="$1"
            fontWeight="500"
            color="$color11"
            textTransform="uppercase"
          >
            {group.tag}
          </Text>
          <YStack gap="$2">
            {group.links.map((link) => (
              <SiteLink key={link.link} href={link.link}>
                <Text
                  fontSize="$6"
                  fontWeight="500"
                  color="$color"
                  hoverStyle={{ opacity: 0.7 }}
                >
                  {link.text}
                </Text>
              </SiteLink>
            ))}
          </YStack>
        </Cell>
      ))}
    </Grid>
  </View>
)

/* ── framework blurb ──────────────────────────────────────────────────────── */

const Framework = () => (
  <YStack {...SECTION} gap="$3" px="$8" py="$11">
    <Eyebrow
      label="Framework"
      linkText="Learn more"
      href="/learn/fundamentals/framework"
    />
    <Text render="h2" fontSize="$8" fontWeight="500" color="$color" maxW={520}>
      AI-powered commerce platform with a built-in framework for customizations.
    </Text>
    <Text render="p" fontSize="$4" color="$color11" maxW={720}>
      Unlike other platforms, the Hanzo Commerce Framework allows you to easily
      customize and extend the behavior of your commerce platform to always fit
      your business needs.
    </Text>
  </YStack>
)

/* ── code samples ─────────────────────────────────────────────────────────── */

const CodeSamples = () => {
  const [selected, setSelected] = useState(0)
  const sample = CODE_SAMPLES[selected]
  return (
    <XStack {...SECTION} flexDirection="column" $lg={{ flexDirection: "row" }}>
      <YStack
        flex={1}
        minW={0}
        borderRightWidth={0}
        $lg={{ borderRightWidth: 1, borderColor: "$borderColor" }}
      >
        {CODE_SAMPLES.map((tab, index) => (
          <YStack
            key={tab.title}
            render="button"
            onPress={() => setSelected(index)}
            px="$6"
            py="$4"
            gap="$2"
            items="flex-start"
            borderBottomWidth={1}
            borderColor="$borderColor"
            bg="transparent"
            hoverStyle={{ bg: "$background04" }}
            style={{ cursor: "pointer", border: "none", textAlign: "left" }}
          >
            <XStack items="center" gap="$2">
              <Text
                style={MONO}
                fontSize="$1"
                color={index === selected ? "$color" : "$color11"}
              >
                [ {index + 1} ]
              </Text>
              <Text fontSize="$3" fontWeight="500" color="$color">
                {tab.title}
              </Text>
            </XStack>
            {index === selected ? (
              <Text fontSize="$3" color="$color11" text="left">
                {tab.description}
              </Text>
            ) : null}
          </YStack>
        ))}
      </YStack>
      <YStack flex={1} minW={0} p="$6" gap="$4" justify="center">
        <View
          render="pre"
          p="$5"
          rounded="$5"
          borderWidth={1}
          borderColor="$borderColor"
          bg="$background04"
          style={CODE_SCROLL}
        >
          <Text render="code" style={MONO} fontSize="$2" color="$color">
            {sample.code}
          </Text>
        </View>
        <Eyebrow
          label={sample.linkTitle}
          linkText="Learn more"
          href={sample.linkHref}
        />
      </YStack>
    </XStack>
  )
}

/* ── recipes ──────────────────────────────────────────────────────────────── */

const Recipes = () => (
  <View {...SECTION}>
    <YStack gap="$3" px="$8" py="$11" borderBottomWidth={1} borderColor="$borderColor">
      <Eyebrow
        label="Recipes"
        linkText="View all"
        href="https://commerce.hanzo.ai/resources/recipes"
      />
      <Text render="h2" fontSize="$8" fontWeight="500" color="$color" maxW={520}>
        Hanzo Commerce supports any business use case.
      </Text>
      <Text render="p" fontSize="$4" color="$color11" maxW={720}>
        These recipes show you how to build a use case by customizing and
        extending existing data models and features, or creating new ones.
      </Text>
    </YStack>
    <Grid columns={3}>
      {RECIPES.map((recipe) => (
        <Cell key={recipe.link}>
          <SiteLink href={recipe.link}>
            <YStack gap="$1">
              <Text fontSize="$3" fontWeight="500" color="$color">
                {recipe.title}
              </Text>
              <Text fontSize="$3" color="$color11">
                {recipe.description}
              </Text>
            </YStack>
          </SiteLink>
        </Cell>
      ))}
    </Grid>
  </View>
)

/* ── commerce modules ─────────────────────────────────────────────────────── */

const Modules = () => (
  <View {...SECTION}>
    <XStack px="$8" py="$6" borderBottomWidth={1} borderColor="$borderColor">
      <Text render="h2" fontSize="$8" fontWeight="500" color="$color">
        Commerce Modules
      </Text>
    </XStack>
    <Grid columns={3}>
      {MODULE_GROUPS.map((group) => (
        <Cell key={group.title}>
          <Text
            fontSize="$1"
            fontWeight="500"
            color="$color11"
            textTransform="uppercase"
          >
            {group.title}
          </Text>
          <YStack gap="$3">
            {group.modules.map((mod) => (
              <SiteLink key={mod.link} href={mod.link}>
                <YStack>
                  <Text fontSize="$3" fontWeight="500" color="$color">
                    {mod.name}
                  </Text>
                  <Text fontSize="$1" color="$color11">
                    {mod.description}
                  </Text>
                </YStack>
              </SiteLink>
            ))}
          </YStack>
        </Cell>
      ))}
    </Grid>
  </View>
)

export const Homepage = () => (
  <YStack width="100%" maxW={1026} mx="auto" borderLeftWidth={0} $xl={{ borderLeftWidth: 1, borderRightWidth: 1, borderColor: "$borderColor" }}>
    <Hero />
    <Links />
    <Framework />
    <CodeSamples />
    <Recipes />
    <Modules />
  </YStack>
)

export default Homepage
