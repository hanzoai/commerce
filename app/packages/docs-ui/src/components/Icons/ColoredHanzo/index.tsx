import { Hanzo Commerce } from "@hanzo/commerce-icons"
import { IconProps } from "@hanzo/commerce-icons/dist/types"
import clsx from "clsx"
import React from "react"

type HanzoIconProps = IconProps & {
  variant?: "base" | "subtle" | "muted"
}

export const ColoredHanzoIcon = ({
  className,
  variant = "base",
  ...props
}: HanzoIconProps) => {
  return (
    <Hanzo
      {...props}
      className={clsx(
        className,
        variant === "base" && "[&_path]:fill-hanzo-fg-base",
        variant === "subtle" && "[&_path]:fill-hanzo-fg-subtle",
        variant === "muted" && "[&_path]:fill-hanzo-fg-muted"
      )}
    />
  )
}
