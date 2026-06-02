"use client"

import React from "react"
import clsx from "clsx"
import { CopyButton } from "@/components/CopyButton"

export type InlineCodeProps = React.ComponentProps<"code"> & {
  variant?: "default" | "grey-bg"
}

export const InlineCode = ({
  variant = "default",
  ...props
}: InlineCodeProps) => {
  return (
    <CopyButton
      text={props.children as string}
      buttonClassName={clsx(
        "bg-transparent border-0 p-0 inline text-hanzo-fg-subtle group",
        "font-monospace"
      )}
    >
      <code
        {...props}
        className={clsx(
          "text-hanzo-tag-neutral-text border whitespace-break-spaces",
          "font-monospace text-code-label rounded-docs_sm py-0 px-[5px]",
          variant === "default" && [
            "bg-hanzo-tag-neutral-bg group-hover:bg-hanzo-tag-neutral-bg-hover",
            "group-active:bg-hanzo-bg-subtle-pressed group-focus:bg-hanzo-bg-subtle-pressed",
            "border-hanzo-tag-neutral-border",
          ],
          variant === "grey-bg" && [
            "bg-hanzo-bg-switch-off group-hover:bg-hanzo-bg-switch-off-hover",
            "group-active:bg-hanzo-bg-switch-off-hover group-focus:bg-hanzo-switch-off-hover",
            "border-hanzo-border-strong",
          ],
          props.className
        )}
      />
    </CopyButton>
  )
}
