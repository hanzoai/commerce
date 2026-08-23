/**
 * The docs kit's server-side face — the ONE module MDX and pages import from.
 *
 * Everything it exports is defined in `./parts`, which is a client module
 * because Gui cannot be evaluated in the server graph. This file is deliberately
 * NOT a client module, and exists for the two things the boundary will not carry:
 *
 *  1. Dot notation. `<Table.Body>` compiles to a property read on whatever
 *     `Table` is. Across the boundary an exported client component is a client
 *     REFERENCE, and a reference has no properties of its own — `Table.Body`
 *     reads `undefined` and MDX fails the page with "Expected component
 *     `Table.Body` to be defined". The grouping object is built HERE, out of
 *     real functions that render the client parts.
 *
 *  2. Component-valued props. A page writes `icon: BookOpen`, naming a component
 *     from `@hanzo/commerce-icons`; React refuses to serialise a function into a
 *     client component ("Functions cannot be passed directly to Client
 *     Components"). Rendering the icon here turns it into an ELEMENT, which
 *     serialises fine.
 *
 * One name, one definition, and both spellings resolve.
 */
import { createElement, isValidElement, type ComponentType, type ReactNode } from "react"

import {
  CardListClient,
  TableBody,
  TableCell,
  TableHeader,
  TableHeaderCell,
  TablePagination,
  TableRoot,
  TableRow,
  TableToolbar,
  type CardProps,
} from "./parts"

export * from "./parts"

/* ── table ────────────────────────────────────────────────────────────────── */

const Root = ({ children }: { children?: ReactNode }) => (
  <TableRoot>{children}</TableRoot>
)

export const Table = Object.assign(Root, {
  Table: Root,
  Header: TableHeader,
  Body: TableBody,
  Row: TableRow,
  HeaderCell: TableHeaderCell,
  Cell: TableCell,
  Toolbar: TableToolbar,
  Pagination: TablePagination,
})

/* ── card list ────────────────────────────────────────────────────────────── */

type IconInput = ComponentType<{ size?: number | string; color?: string }> | ReactNode

/** An element for whatever a page put in `icon` — a component, an element, or nothing. */
function renderIcon(icon: IconInput): ReactNode {
  if (icon === null || icon === undefined || typeof icon === "boolean") return null
  if (isValidElement(icon)) return icon
  if (typeof icon === "function" || typeof icon === "object") {
    return createElement(icon as ComponentType<{ size?: number }>, { size: 16 })
  }
  return null
}

export type CardListItem = Omit<CardProps, "icon"> & { icon?: IconInput }

export const CardList = ({
  items,
  ...rest
}: {
  items: CardListItem[]
  itemsPerRow?: number
  defaultItemsPerRow?: number
  className?: string
}) => (
  <CardListClient
    {...rest}
    items={items.map((item) => ({ ...item, icon: renderIcon(item.icon) }))}
  />
)
