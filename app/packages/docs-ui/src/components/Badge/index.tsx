import React from "react"
import clsx from "clsx"
import { ShadedBgIcon } from "../Icons/ShadedBg"

export type BadgeVariant =
  | "purple"
  | "orange"
  | "green"
  | "blue"
  | "red"
  | "neutral"
  | "code"

export type BadgeType = "default" | "shaded"

export type BadgeProps = {
  className?: string
  childrenWrapperClassName?: string
  variant: BadgeVariant
  badgeType?: BadgeType
} & React.HTMLAttributes<HTMLSpanElement>

export const Badge = ({
  className,
  variant,
  badgeType = "default",
  children,
  childrenWrapperClassName,
  ...props
}: BadgeProps) => {
  return (
    <span
      className={clsx(
        "text-compact-x-small-plus text-center",
        badgeType === "default" &&
          "px-docs_0.25 py-0 rounded-docs_sm border border-solid",
        variant === "purple" &&
          "bg-hanzo-tag-purple-bg text-hanzo-tag-purple-text border-hanzo-tag-purple-border",
        variant === "orange" &&
          "bg-hanzo-tag-orange-bg text-hanzo-tag-orange-text border-hanzo-tag-orange-border",
        variant === "green" &&
          "bg-hanzo-tag-green-bg text-hanzo-tag-green-text border-hanzo-tag-green-border",
        variant === "blue" &&
          "bg-hanzo-tag-blue-bg text-hanzo-tag-blue-text border-hanzo-tag-blue-border",
        variant === "red" &&
          "bg-hanzo-tag-red-bg text-hanzo-tag-red-text border-hanzo-tag-red-border",
        variant === "neutral" &&
          "bg-hanzo-tag-neutral-bg text-hanzo-tag-neutral-text border-hanzo-tag-neutral-border",
        variant === "code" &&
          "bg-hanzo-contrast-bg-subtle text-hanzo-contrast-fg-secondary border-hanzo-contrast-border-bot",
        badgeType === "shaded" && "px-[3px] !bg-transparent relative",
        // needed for tailwind utilities
        "badge",
        className
      )}
      {...props}
    >
      {badgeType === "shaded" && (
        <ShadedBgIcon
          variant={variant}
          className={clsx("absolute top-0 left-0 w-full h-full")}
        />
      )}
      <span
        className={clsx(
          badgeType === "shaded" && "relative z-[1]",
          childrenWrapperClassName
        )}
      >
        {children}
      </span>
    </span>
  )
}
