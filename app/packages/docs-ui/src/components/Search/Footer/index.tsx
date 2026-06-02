import clsx from "clsx"
import React from "react"
import { Kbd } from "@/components/Kbd"

export const SearchFooter = () => {
  return (
    <div
      className={clsx(
        "py-docs_0.75 hidden md:flex items-center justify-end px-docs_1",
        "border-hanzo-border-base border-t",
        "bg-hanzo-bg-field z-10"
      )}
    >
      <div className="flex items-center gap-docs_0.75">
        <div className="flex items-center gap-docs_0.5">
          <span
            className={clsx(
              "text-hanzo-fg-subtle",
              "text-compact-x-small-plus"
            )}
          >
            Navigation
          </span>
          <span className="gap-[5px] flex">
            <Kbd
              className={clsx(
                "!bg-hanzo-bg-field-component !border-hanzo-border-strong",
                "!text-hanzo-fg-subtle h-[18px] w-[18px] p-0"
              )}
            >
              ↑
            </Kbd>
            <Kbd
              className={clsx(
                "!bg-hanzo-bg-field-component !border-hanzo-border-strong",
                "!text-hanzo-fg-subtle h-[18px] w-[18px] p-0"
              )}
            >
              ↓
            </Kbd>
          </span>
        </div>
        <div className={clsx("h-docs_0.75 w-px bg-hanzo-border-strong")}></div>
        <div className="flex items-center gap-docs_0.5">
          <span
            className={clsx(
              "text-hanzo-fg-subtle",
              "text-compact-x-small-plus"
            )}
          >
            Open Result
          </span>
          <Kbd
            className={clsx(
              "!bg-hanzo-bg-field-component !border-hanzo-border-strong",
              "!text-hanzo-fg-subtle h-[18px] w-[18px] p-0"
            )}
          >
            ↵
          </Kbd>
        </div>
      </div>
    </div>
  )
}
