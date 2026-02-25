import React from "react"

import type { LinkProps as NextLinkProps } from "next/link"
import Link from "next/link"
import clsx from "clsx"

type LinkButtonProps = NextLinkProps & {
  variant?: "base" | "interactive" | "subtle" | "muted"
  className?: string
} & React.AllHTMLAttributes<HTMLAnchorElement>

export const LinkButton = ({
  variant = "base",
  className,
  ...linkProps
}: LinkButtonProps) => {
  return (
    <Link
      {...linkProps}
      className={clsx(
        className,
        "inline-flex justify-center items-center",
        "gap-docs_0.25 rounded-docs_xs",
        "text-compact-small-plus disabled:text-hanzo-fg-disabled",
        "focus:shadow-borders-focus no-underline",
        variant === "base" && [
          "text-hanzo-fg-base hover:text-hanzo-fg-subtle",
          "focus:text-hanzo-fg-base",
        ],
        variant === "interactive" && [
          "text-hanzo-fg-interactive hover:text-hanzo-interactive-hover",
          "focus:text-hanzo-fg-interactive",
        ],
        variant === "subtle" && [
          "text-hanzo-fg-subtle hover:text-hanzo-fg-base",
          "focus:text-hanzo-fg-subtle",
        ],
        variant === "muted" && [
          "text-hanzo-fg-muted hover:text-hanzo-fg-subtle",
          "focus:text-hanzo-fg-muted",
        ]
      )}
    />
  )
}
