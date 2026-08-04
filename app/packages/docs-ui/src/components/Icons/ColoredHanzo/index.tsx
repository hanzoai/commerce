import { HanzoLogo } from "@hanzo/logo/react"
import clsx from "clsx"
import React from "react"

type ColoredHanzoIconProps = {
  variant?: "base" | "subtle" | "muted"
  className?: string
}

/**
 * The Hanzo mark, at one of the docs foreground weights.
 *
 * It comes from `@hanzo/logo` — the one package that owns the mark on every
 * Hanzo surface — rather than from a logomark vendored into this repo's icon
 * set. The mono variant inherits `currentColor`, so weight is a text colour.
 */
export const ColoredHanzoIcon = ({
  className,
  variant = "base",
}: ColoredHanzoIconProps) => (
  <HanzoLogo
    variant="mono"
    className={clsx(
      className,
      variant === "base" && "text-hanzo-fg-base",
      variant === "subtle" && "text-hanzo-fg-subtle",
      variant === "muted" && "text-hanzo-fg-muted"
    )}
  />
)
