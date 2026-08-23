"use client"

/**
 * The docs component kit — every shape an MDX page can reach for, once.
 *
 * Everything here is a CLIENT component, and has to be: Gui builds its theme
 * and style contexts with `React.createContext` at module scope, and the React
 * that Next resolves in the SERVER graph (the `react-server` condition) has no
 * `createContext`. A Gui component evaluated as a server component takes the
 * whole static export down with "r is not a function".
 *
 * `index.tsx` is the server-side face of this file — see the note there about
 * why `Table.Body` cannot live on a client reference.
 *
 * The pages used to import these from `@hanzo/commerce-docs-ui`, a 458-file
 * Tailwind fork of a documentation theme that carried its own colour scale, its
 * own Tailwind preset and its own copy of a component library. This module is
 * that surface rebuilt on `@hanzo/gui` style props and the shared
 * `@hanzo/ui/gui-config` scale, so the docs, the Commerce admin and the console
 * draw from ONE token set. No Tailwind, no Radix, no second scale.
 *
 * Gui pitfalls this file is written against — each one fails SILENTLY, because
 * an unrecognised prop is dropped rather than rejected:
 *   · `render`, not `tag`, names the host element.
 *   · `$sm`/`$lg` are the mobile-first min-width keys; `$gtSm` does not exist.
 *   · a bare number `lineHeight` is PIXELS — a ratio must be a string, on `style`.
 *   · `letterSpacing: var(...)` is not a legal token prop — it rides `style`.
 *   · `transition`, not `animation`, is the 8.x easing prop.
 *   · `display: grid` is not in Gui's RN-derived union — it rides `style`.
 */

import React, {
  Children,
  createContext,
  isValidElement,
  useContext,
  useId,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react"
import NextLink from "next/link"
import { Text, View, XStack, YStack } from "@hanzo/gui"
import { Button as UiButton } from "@hanzo/ui"

/* ── shared values with no token behind them ──────────────────────────────── */

/** CSS grid — Gui's `display` union is React-Native-derived and stops at flex. */
const GRID: CSSProperties = { display: "grid" }
/** A ratio, not a length: Gui appends `px` to a bare number, prop or style alike. */
const TIGHT_LEADING: CSSProperties = { lineHeight: "1.25" }

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

const BORDER = { borderWidth: 1, borderColor: "$borderColor" } as const

/* ── headings ─────────────────────────────────────────────────────────────── */

type HeadingProps = { children?: ReactNode; id?: string }

export const H1 = ({ children, id }: HeadingProps) => (
  <Text render="h1" id={id} fontSize="$10" fontWeight="500" color="$color" mt="$6" mb="$3" style={TIGHT_LEADING}>
    {children}
  </Text>
)

export const H2 = ({ children, id }: HeadingProps) => (
  <Text render="h2" id={id} fontSize="$8" fontWeight="500" color="$color" mt="$6" mb="$3" style={TIGHT_LEADING}>
    {children}
  </Text>
)

export const H3 = ({ children, id }: HeadingProps) => (
  <Text render="h3" id={id} fontSize="$7" fontWeight="500" color="$color" mt="$6" mb="$3" style={TIGHT_LEADING}>
    {children}
  </Text>
)

/* ── link + button ────────────────────────────────────────────────────────── */

export type LinkProps = {
  href?: string
  /** The docs' own spelling for an internal destination. */
  to?: string
  children?: ReactNode
  target?: string
  rel?: string
  variant?: string
}

/**
 * One link. `next/link` for anything in-site (it keeps the client router and the
 * export's trailing-slash contract), a plain anchor for anything that leaves.
 */
export const Link = ({ href, to, children, target, rel }: LinkProps) => {
  const dest = href ?? to ?? "#"
  const external = /^[a-z]+:/i.test(dest) || target === "_blank"
  if (external) {
    return (
      <a href={dest} target={target} rel={rel ?? "noopener noreferrer"}>
        {children}
      </a>
    )
  }
  return <NextLink href={dest}>{children}</NextLink>
}

/** The docs only ever want the shared button; it is re-exported so MDX has one. */
export const Button = UiButton

/* ── icons ────────────────────────────────────────────────────────────────── */

export type DocsIcon = React.ComponentType<{
  size?: number | string
  color?: string
  className?: string
}>

/** The AI-assistant glyph the docs point at in prose. */
export const BloomIcon: DocsIcon = ({ size = 16, color = "currentColor" }) => (
  <svg width={size} height={size} viewBox="0 0 20 20" fill="none" aria-hidden>
    <path
      d="M10 2.5 11.6 7 16 8.6 11.6 10.2 10 14.7 8.4 10.2 4 8.6 8.4 7 10 2.5Z"
      fill={color}
    />
    <circle cx="15.2" cy="14.8" r="1.8" fill={color} opacity="0.7" />
  </svg>
)

/** An icon set inline in a sentence, sitting on the text baseline. */
export const InlineIcon = ({
  Icon,
  alt,
}: {
  Icon: DocsIcon
  alt?: string
}) => (
  <View
    render="span"
    display="inline-flex"
    items="center"
    style={{ verticalAlign: "-0.15em" }}
    aria-label={alt}
  >
    <Icon size={16} color="currentColor" />
  </View>
)

/* ── card + card list ─────────────────────────────────────────────────────── */

export type CardProps = {
  title?: string
  text?: string
  href?: string
  /**
   * An already-rendered glyph, not a component.
   *
   * A page names an icon COMPONENT (`icon: BookOpen`), and a component cannot
   * cross the server/client boundary as a prop — React refuses to serialise it.
   * `./index.tsx` renders it in the server graph and hands the element down;
   * an element serialises fine.
   */
  icon?: ReactNode
  children?: ReactNode
}

export const Card = ({ title, text, href, icon, children }: CardProps) => {
  const body = (
    <YStack
      {...BORDER}
      rounded="$5"
      p="$4"
      gap="$2"
      height="100%"
      bg="$background"
      hoverStyle={{ borderColor: "$colorHover" }}
    >
      {title ? (
        <XStack items="center" gap="$2">
          {icon}
          <Text fontSize="$4" fontWeight="500" color="$color">
            {title}
          </Text>
        </XStack>
      ) : null}
      {text ? (
        <Text fontSize="$3" color="$color11">
          {text}
        </Text>
      ) : null}
      {children}
    </YStack>
  )
  return href ? (
    <Link href={href}>{body}</Link>
  ) : (
    body
  )
}

/**
 * A responsive card grid. One column on a phone, `itemsPerRow` from `$md` up —
 * a real CSS grid, so tiles in a row share a height instead of each sizing to
 * its own copy.
 *
 * `./index.tsx` exports the `CardList` a page actually names; it renders each
 * item's icon before handing the list here (see `CardProps.icon`).
 */
export const CardListClient = ({
  items,
  itemsPerRow,
  defaultItemsPerRow,
}: {
  items: CardProps[]
  itemsPerRow?: number
  defaultItemsPerRow?: number
  className?: string
}) => {
  const cols =
    itemsPerRow ??
    (items.length === 1
      ? 1
      : defaultItemsPerRow ?? (items.length % 2 === 0 ? 2 : 3))
  return (
    <View
      render="section"
      style={GRID}
      gap="$4"
      my="$4"
      gridTemplateColumns="1fr"
      $md={{
        gridTemplateColumns:
          cols === 1 ? "1fr" : `repeat(${Math.min(cols, 3)}, minmax(0, 1fr))`,
      }}
    >
      {items.map((item, i) => (
        <Card key={i} {...item} />
      ))}
    </View>
  )
}

/* ── prerequisites ────────────────────────────────────────────────────────── */

export type PrerequisiteItem = { text: string; link?: string }

/** What a chapter assumes you already did, as a plain list of links. */
export const Prerequisites = ({ items }: { items: PrerequisiteItem[] }) => (
  <YStack {...BORDER} rounded="$5" p="$4" my="$4" gap="$2" bg="$background">
    <Text fontSize="$2" fontWeight="500" color="$color11">
      Prerequisites
    </Text>
    <YStack render="ul" gap="$1" style={{ listStyle: "none", padding: 0, margin: 0 }}>
      {items.map((item, i) => (
        <Text render="li" key={i} fontSize="$3" color="$color">
          {item.link ? <Link href={item.link}>{item.text}</Link> : item.text}
        </Text>
      ))}
    </YStack>
  </YStack>
)


/* ── note ─────────────────────────────────────────────────────────────────── */

/**
 * A callout. `type` is the docs' own vocabulary (`check`, `warning`, `error`,
 * `soon`); it selects an accent from the token set and nothing else, so a type
 * nobody defined still renders a readable note instead of an invisible one.
 */
const NOTE_ACCENT = {
  check: "$green9",
  success: "$green9",
  warning: "$yellow9",
  error: "$red9",
  soon: "$blue9",
} as const

type NoteType = keyof typeof NOTE_ACCENT

export const Note = ({
  type,
  title,
  children,
}: {
  type?: string
  title?: string
  children?: ReactNode
}) => (
  <YStack
    my="$4"
    p="$4"
    gap="$2"
    rounded="$5"
    bg="$background04"
    borderLeftWidth={3}
    borderColor={NOTE_ACCENT[type as NoteType] ?? "$borderColor"}
  >
    {title ? (
      <Text fontSize="$2" fontWeight="500" color="$color">
        {title}
      </Text>
    ) : null}
    {children}
  </YStack>
)

/* ── details ──────────────────────────────────────────────────────────────── */

/**
 * A collapsible aside. The native <details> element already IS this control —
 * it opens without JavaScript and it is what a screen reader announces — so the
 * kit styles it rather than reimplementing it.
 */
export const Details = ({
  summaryContent,
  openInitial = false,
  children,
}: {
  summaryContent?: ReactNode
  openInitial?: boolean
  children?: ReactNode
}) => (
  <View
    render="details"
    {...BORDER}
    rounded="$5"
    p="$3"
    my="$4"
    bg="$background"
    {...({ open: openInitial } as Record<string, unknown>)}
  >
    <Text
      render="summary"
      fontSize="$3"
      fontWeight="500"
      color="$color"
      style={{ cursor: "pointer" }}
    >
      {summaryContent ?? "Details"}
    </Text>
    <YStack mt="$3">{children}</YStack>
  </View>
)

/* ── table ────────────────────────────────────────────────────────────────── */

export type CellProps = { children?: ReactNode; className?: string }

export const TableRoot = ({ children }: CellProps) => (
  <View my="$4" overflow="hidden" {...BORDER} rounded="$5">
    <View
      render="table"
      width="100%"
      style={{ borderCollapse: "collapse", tableLayout: "auto" }}
    >
      {children}
    </View>
  </View>
)

export const TableHeader = ({ children }: CellProps) => (
  <View render="thead" bg="$background04">
    {children}
  </View>
)
export const TableBody = ({ children }: CellProps) => (
  <View render="tbody">{children}</View>
)
export const TableRow = ({ children }: CellProps) => (
  <View render="tr" borderBottomWidth={1} borderColor="$borderColor">
    {children}
  </View>
)
export const TableHeaderCell = ({ children }: CellProps) => (
  <Text
    render="th"
    px="$3"
    py="$2"
    fontSize="$2"
    fontWeight="500"
    color="$color11"
    text="left"
  >
    {children}
  </Text>
)
export const TableCell = ({ children }: CellProps) => (
  <Text render="td" px="$3" py="$2" fontSize="$3" color="$color" text="left">
    {children}
  </Text>
)

/**
 * `Table.Toolbar` and `Table.Pagination` exist because pages name them; a static
 * export has nothing to paginate, so they are layout slots and nothing more.
 */
export const TableToolbar = ({ children }: CellProps) => (
  <XStack px="$3" py="$2" items="center" gap="$2">
    {children}
  </XStack>
)

export const TablePagination = ({ children }: CellProps) => (
  <XStack px="$3" py="$2" items="center" justify="flex-end" gap="$2">
    {children}
  </XStack>
)

/* ── type list ────────────────────────────────────────────────────────────── */

export type TypeEntry = {
  name: string
  type?: string
  optional?: boolean
  defaultValue?: string
  description?: string
  children?: TypeEntry[]
}

const TypeRow = ({ entry, depth }: { entry: TypeEntry; depth: number }) => (
  <YStack
    borderTopWidth={1}
    borderColor="$borderColor"
    px="$3"
    py="$2"
    gap="$1"
    style={{ paddingLeft: `calc(0.75rem + ${depth} * 1rem)` }}
  >
    <XStack items="baseline" gap="$2" flexWrap="wrap">
      <Text fontSize="$3" fontWeight="500" color="$color" style={MONO}>
        {entry.name}
      </Text>
      {entry.type ? (
        <Text fontSize="$2" color="$color11" style={MONO}>
          {entry.type}
        </Text>
      ) : null}
      {entry.optional ? (
        <Text fontSize="$1" color="$color11">
          optional
        </Text>
      ) : null}
      {entry.defaultValue ? (
        <Text fontSize="$1" color="$color11">
          default: {entry.defaultValue}
        </Text>
      ) : null}
    </XStack>
    {entry.description ? (
      <Text fontSize="$3" color="$color11">
        {entry.description}
      </Text>
    ) : null}
    {entry.children?.map((child, i) => (
      <TypeRow key={i} entry={child} depth={depth + 1} />
    ))}
  </YStack>
)

/** A parameter/return-value table, flattened to rows so it survives a phone. */
export const TypeList = ({
  types,
  sectionTitle,
}: {
  types: TypeEntry[]
  sectionTitle?: string
  expandUrl?: string
  openedLevel?: number
  className?: string
}) => (
  <YStack {...BORDER} rounded="$5" my="$4" overflow="hidden" bg="$background">
    {sectionTitle ? (
      <Text px="$3" py="$2" fontSize="$2" fontWeight="500" color="$color11">
        {sectionTitle}
      </Text>
    ) : null}
    {types.map((entry, i) => (
      <TypeRow key={i} entry={entry} depth={0} />
    ))}
  </YStack>
)

/* ── tabs ─────────────────────────────────────────────────────────────────── */

type TabsCtx = {
  value: string
  setValue: (v: string) => void
  vertical: boolean
}
const TabsContext = createContext<TabsCtx | null>(null)

const useTabs = (): TabsCtx => {
  const ctx = useContext(TabsContext)
  if (!ctx) {
    throw new Error("A tab part must be rendered inside <Tabs>.")
  }
  return ctx
}

export const Tabs = ({
  defaultValue,
  layoutType = "horizontal",
  children,
}: {
  defaultValue?: string
  layoutType?: "horizontal" | "vertical"
  className?: string
  children?: ReactNode
}) => {
  const [value, setValue] = useState(defaultValue ?? "")
  const vertical = layoutType === "vertical"
  return (
    <TabsContext.Provider value={{ value, setValue, vertical }}>
      {vertical ? (
        <YStack my="$4" gap="$3" $md={{ flexDirection: "row" }}>
          {children}
        </YStack>
      ) : (
        <YStack my="$4" gap="$3">
          {children}
        </YStack>
      )}
    </TabsContext.Provider>
  )
}

export const TabsList = ({ children }: { children?: ReactNode }) => {
  const { vertical } = useTabs()
  return vertical ? (
    <YStack
      gap="$1"
      minW={180}
      borderRightWidth={0}
      $md={{ borderRightWidth: 1, borderColor: "$borderColor", pr: "$2" }}
    >
      {children}
    </YStack>
  ) : (
    <XStack gap="$1" borderBottomWidth={1} borderColor="$borderColor" flexWrap="wrap">
      {children}
    </XStack>
  )
}

const Trigger = ({
  value,
  children,
  vertical,
}: {
  value: string
  children?: ReactNode
  vertical: boolean
}) => {
  const tabs = useTabs()
  const active = tabs.value === value
  return (
    <Text
      render="button"
      onPress={() => tabs.setValue(value)}
      px="$3"
      py="$2"
      fontSize="$3"
      text={vertical ? "left" : "center"}
      color={active ? "$color" : "$color11"}
      bg={active && vertical ? "$background04" : "transparent"}
      rounded={vertical ? "$3" : 0}
      borderBottomWidth={vertical ? 0 : 2}
      borderColor={active && !vertical ? "$color" : "transparent"}
      hoverStyle={{ color: "$color" }}
      style={{ cursor: "pointer", border: vertical ? "none" : undefined }}
    >
      {children}
    </Text>
  )
}

export const TabsTrigger = (props: { value: string; children?: ReactNode }) => (
  <Trigger {...props} vertical={false} />
)

export const TabsTriggerVertical = (props: {
  value: string
  children?: ReactNode
}) => <Trigger {...props} vertical />

export const TabsContentWrapper = ({ children }: { children?: ReactNode; className?: string }) => (
  <YStack flex={1} minW={0}>
    {children}
  </YStack>
)

export const TabsContent = ({
  value,
  children,
}: {
  value: string
  children?: ReactNode
}) => {
  const tabs = useTabs()
  if (tabs.value !== value) {
    return null
  }
  return <YStack>{children}</YStack>
}

/* ── code tabs ────────────────────────────────────────────────────────────── */

type CodeTabProps = { label: string; value: string; children?: ReactNode }

/**
 * One labelled code sample. It is a marker: `CodeTabs` reads `label`/`value` off
 * it and renders the children itself, so a page reads as the tab strip it is.
 */
export const CodeTab = ({ children }: CodeTabProps) => <>{children}</>

/** A code sample shown in one of several flavours (npm/yarn, JS/TS, …). */
export const CodeTabs = ({
  children,
  group,
}: {
  children?: ReactNode
  className?: string
  group?: string
  blockStyle?: string
}) => {
  const tabs = Children.toArray(children).filter(
    (child): child is React.ReactElement<CodeTabProps> =>
      isValidElement(child) && typeof child.props === "object"
  )
  const first = tabs[0]?.props.value ?? ""
  const [value, setValue] = useState(first)
  const id = useId()
  const shown = tabs.find((t) => t.props.value === value) ?? tabs[0]
  return (
    <YStack my="$4" {...BORDER} rounded="$5" overflow="hidden" bg="$background">
      <XStack
        borderBottomWidth={1}
        borderColor="$borderColor"
        bg="$background04"
        flexWrap="wrap"
        aria-label={group}
      >
        {tabs.map((tab) => {
          const active = tab.props.value === (shown?.props.value ?? "")
          return (
            <Text
              key={`${id}-${tab.props.value}`}
              render="button"
              onPress={() => setValue(tab.props.value)}
              px="$3"
              py="$2"
              fontSize="$2"
              color={active ? "$color" : "$color11"}
              borderBottomWidth={2}
              borderColor={active ? "$color" : "transparent"}
              hoverStyle={{ color: "$color" }}
              style={{ cursor: "pointer", background: "none", border: "none" }}
            >
              {tab.props.label}
            </Text>
          )
        })}
      </XStack>
      <YStack px="$3">{shown?.props.children}</YStack>
    </YStack>
  )
}

/* ── split sections ───────────────────────────────────────────────────────── */

/** A two-column narrative block: prose on the left, its example on the right. */
export const SplitSections = ({ children }: { children?: ReactNode }) => (
  <YStack gap="$8" my="$4">
    {children}
  </YStack>
)

export const SplitSection = ({
  content,
  code,
  children,
}: {
  content?: ReactNode
  code?: ReactNode
  children?: ReactNode
}) => (
  <YStack gap="$4" $lg={{ flexDirection: "row" }}>
    <YStack flex={1} minW={0}>
      {content ?? children}
    </YStack>
    {code ? (
      <YStack flex={1} minW={0}>
        {code}
      </YStack>
    ) : null}
  </YStack>
)

/** A link list laid out in `listsNum` columns. */
export const SplitList = ({
  items,
  listsNum = 1,
}: {
  items: { title: string; link: string }[]
  listsNum?: number
}) => (
  <View
    style={GRID}
    gap="$2"
    my="$4"
    gridTemplateColumns="1fr"
    $md={{ gridTemplateColumns: `repeat(${listsNum}, minmax(0, 1fr))` }}
  >
    {items.map((item) => (
      <Text key={item.link} fontSize="$3" color="$color">
        <Link href={item.link}>{item.title}</Link>
      </Text>
    ))}
  </View>
)

/* ── pages that need a docs graph ─────────────────────────────────────────── */

/**
 * `ChildDocs` and `SimilarPages` listed sibling pages from an index the deleted
 * docs fork built at compile time. There is no such index in this export, and a
 * hard-coded stand-in would be a lie on every page that renders one — so they
 * render nothing until a real index exists.
 */
export const ChildDocs = (_props: { type?: string; onlyTopLevel?: boolean }) =>
  null

export const SimilarPages = () => null
