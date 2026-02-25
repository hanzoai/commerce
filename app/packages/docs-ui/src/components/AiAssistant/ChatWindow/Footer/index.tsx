import React from "react"
import clsx from "clsx"
import { Kbd } from "../../../Kbd"
import { KapaIcon } from "../../../Icons/Kapa"
import { Tooltip } from "../../../Tooltip"

export const AiAssistantChatWindowFooter = () => {
  return (
    <div
      className={clsx(
        "bg-hanzo-bg-component border-t border-hanzo-border-base",
        "flex items-center justify-between gap-docs_0.75 text-compact-x-small",
        "py-docs_0.75 px-docs_1"
      )}
    >
      <Tooltip text="The docs AI assistant is powered by Kapa.ai">
        <a href="https://kapa.ai" target="_blank" rel="noreferrer">
          <KapaIcon className="text-hanzo-fg-disabled hover:text-hanzo-fg-muted transition-colors" />
        </a>
      </Tooltip>
      <div className="flex items-center justify-end gap-docs_0.75">
        <span className="text-hanzo-fg-muted">Chat is cleared on refresh</span>
        <span className="h-docs_0.75 w-px bg-hanzo-border-base"></span>
        <div className="flex items-center gap-docs_0.5">
          <span className="text-hanzo-fg-subtle">Line break</span>
          <div className="flex items-center gap-[5px]">
            <Kbd className="bg-hanzo-bg-field-component border-hanzo-border-strong w-[18px] h-[18px] inline-block p-0">
              ⇧
            </Kbd>
            <Kbd className="bg-hanzo-bg-field-component border-hanzo-border-strong w-[18px] h-[18px] inline-block p-0">
              ↵
            </Kbd>
          </div>
        </div>
      </div>
    </div>
  )
}
