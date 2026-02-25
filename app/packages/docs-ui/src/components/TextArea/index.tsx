import React from "react"
import clsx from "clsx"

export type TextAreaProps = {
  className?: string
} & React.DetailedHTMLProps<
  React.TextareaHTMLAttributes<HTMLTextAreaElement>,
  HTMLTextAreaElement
>

export const TextArea = (props: TextAreaProps) => {
  return (
    <textarea
      {...props}
      className={clsx(
        "bg-hanzo-bg-field shadow-border-base dark:shadow-border-base-dark",
        "rounded-docs_sm",
        "py-[6px] px-docs_0.5 text-medium font-base",
        "hover:bg-hanzo-bg-field-hover",
        "focus:shadow-hanzo-border-interactive-with-focus",
        "active:shadow-hanzo-border-interactive-with-focus",
        "disabled:bg-hanzo-bg-disabled",
        "disabled:border-hanzo-border-base disabled:border disabled:shadow-none",
        "placeholder:text-hanzo-fg-muted",
        "disabled:placeholder:text-hanzo-fg-disabled",
        props.className
      )}
    />
  )
}
