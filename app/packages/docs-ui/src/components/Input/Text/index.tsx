import React from "react"
import clsx from "clsx"

export type InputTextProps = {
  className?: string
  addGroupStyling?: boolean
  passedRef?: React.Ref<HTMLInputElement>
} & React.DetailedHTMLProps<
  React.InputHTMLAttributes<HTMLInputElement>,
  HTMLInputElement
>

export const InputText = ({
  addGroupStyling = false,
  className,
  passedRef,
  ...props
}: InputTextProps) => {
  return (
    <input
      {...props}
      className={clsx(
        "bg-hanzo-bg-field-component shadow-border-base dark:shadow-border-base-dark",
        "rounded-docs_sm px-docs_0.5",
        "hover:bg-hanzo-bg-field-component-hover",
        addGroupStyling && "group-hover:bg-hanzo-bg-field-component-hover",
        "focus:border-hanzo-border-interactive",
        "active:border-hanzo-border-interactive",
        "disabled:bg-hanzo-bg-disabled",
        "disabled:border-hanzo-border-base",
        "placeholder:text-hanzo-fg-muted",
        "disabled:placeholder:text-hanzo-fg-disabled",
        "text-compact-small font-base",
        className
      )}
      ref={passedRef}
    />
  )
}
