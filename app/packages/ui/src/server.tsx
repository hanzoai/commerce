/**
 * Server-safe kit components — plain elements + kit.css on theme tokens.
 *
 * These render in React Server Components (order pages, cart templates,
 * skeletons), so they hold no context, no state and no gui import. Compound
 * property access (Table.Cell) only works on a module evaluated in the same
 * graph as its consumer — a client reference has no properties of its own —
 * which is exactly why Table lives here and not behind "use client".
 */
import * as React from "react"

import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

/** Class joiner. The twMerge half dies with the last Tailwind class. */
export function clx(...args: ClassValue[]): string {
  return twMerge(clsx(...args))
}

/* -- Text ------------------------------------------------------------------ */

type TextSize = "xsmall" | "small" | "base" | "large" | "xlarge"

export interface TextProps extends React.HTMLAttributes<HTMLParagraphElement> {
  size?: TextSize
  weight?: "regular" | "plus"
  family?: "sans" | "mono"
  leading?: "normal" | "compact"
  as?: "p" | "span" | "div"
}

export const Text = React.forwardRef<HTMLParagraphElement, TextProps>(
  (
    { className, size = "base", weight, family, leading, as = "p", ...props },
    ref
  ) => {
    const As = as
    return (
      <As
        ref={ref as React.Ref<never>}
        className={clx(
          "kit-text",
          size !== "base" && `kit-text-${size}`,
          leading === "compact" && "kit-text-compact",
          weight === "plus" && "kit-text-plus",
          family === "mono" && "kit-text-mono",
          className
        )}
        {...props}
      />
    )
  }
)
Text.displayName = "Text"

/* -- Heading ---------------------------------------------------------------- */

export interface HeadingProps extends React.HTMLAttributes<HTMLHeadingElement> {
  level?: "h1" | "h2" | "h3"
}

export const Heading = ({ level = "h1", className, ...props }: HeadingProps) => {
  const As = level
  return <As className={clx(`kit-${level}`, className)} {...props} />
}

/* -- Container --------------------------------------------------------------- */

export const Container = React.forwardRef<
  HTMLDivElement,
  React.ComponentPropsWithoutRef<"div">
>(({ className, ...props }, ref) => (
  <div ref={ref} className={clx("kit-container", className)} {...props} />
))
Container.displayName = "Container"

/* -- Label --------------------------------------------------------------------- */

export interface LabelProps extends React.ComponentPropsWithoutRef<"label"> {
  size?: "xsmall" | "small" | "base" | "large"
  weight?: "regular" | "plus"
}

export const Label = React.forwardRef<HTMLLabelElement, LabelProps>(
  ({ className, size = "base", weight, ...props }, ref) => (
    <label
      ref={ref}
      className={clx(
        "kit-label",
        size !== "base" && `kit-label-${size}`,
        weight === "plus" && "kit-label-plus",
        className
      )}
      {...props}
    />
  )
)
Label.displayName = "Label"

/* -- Input ------------------------------------------------------------------------ */

export type InputProps = React.ComponentPropsWithoutRef<"input">

/** Native input, kit-styled: form fields must land their `name` in the DOM. */
export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, ...props }, ref) => (
    <input ref={ref} className={clx("kit-input", className)} {...props} />
  )
)
Input.displayName = "Input"

/* -- Badge ---------------------------------------------------------------------------- */

export interface BadgeProps
  extends Omit<React.HTMLAttributes<HTMLSpanElement>, "color"> {
  color?: "green" | "red" | "blue" | "orange" | "grey" | "purple"
  size?: "2xsmall" | "xsmall" | "small" | "base" | "large"
  rounded?: "base" | "full"
}

export const Badge = React.forwardRef<HTMLSpanElement, BadgeProps>(
  (
    { className, color = "grey", size = "base", rounded = "base", ...props },
    ref
  ) => (
    <span
      ref={ref}
      className={clx(
        "kit-badge",
        `kit-badge-${color}`,
        size !== "base" && `kit-badge-${size}`,
        rounded === "full" && "kit-badge-full",
        className
      )}
      {...props}
    />
  )
)
Badge.displayName = "Badge"

export const IconBadge = React.forwardRef<
  HTMLSpanElement,
  React.HTMLAttributes<HTMLSpanElement>
>(({ className, ...props }, ref) => (
  <span ref={ref} className={clx("kit-icon-badge", className)} {...props} />
))
IconBadge.displayName = "IconBadge"

/* -- Table (compound, server-rendered) --------------------------------------------------- */

type TableRootProps = React.TableHTMLAttributes<HTMLTableElement>

const TableRoot = React.forwardRef<HTMLTableElement, TableRootProps>(
  ({ className, ...props }, ref) => (
    <table ref={ref} className={clx("kit-table", className)} {...props} />
  )
)
TableRoot.displayName = "Table"

const TableHeader = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <thead ref={ref} className={clx("kit-table-header", className)} {...props} />
))
TableHeader.displayName = "Table.Header"

const TableBody = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>((props, ref) => <tbody ref={ref} {...props} />)
TableBody.displayName = "Table.Body"

const TableRow = React.forwardRef<
  HTMLTableRowElement,
  React.HTMLAttributes<HTMLTableRowElement>
>(({ className, ...props }, ref) => (
  <tr ref={ref} className={clx("kit-table-row", className)} {...props} />
))
TableRow.displayName = "Table.Row"

const TableHeaderCell = React.forwardRef<
  HTMLTableCellElement,
  React.ThHTMLAttributes<HTMLTableCellElement>
>(({ className, ...props }, ref) => (
  <th ref={ref} className={clx("kit-table-header-cell", className)} {...props} />
))
TableHeaderCell.displayName = "Table.HeaderCell"

const TableCell = React.forwardRef<
  HTMLTableCellElement,
  React.TdHTMLAttributes<HTMLTableCellElement>
>(({ className, ...props }, ref) => (
  <td ref={ref} className={clx("kit-table-cell", className)} {...props} />
))
TableCell.displayName = "Table.Cell"

export const Table = Object.assign(TableRoot, {
  Header: TableHeader,
  Body: TableBody,
  Row: TableRow,
  HeaderCell: TableHeaderCell,
  Cell: TableCell,
})
