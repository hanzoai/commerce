"use client"

/**
 * Interactive kit components — @hanzo/ui on @hanzo/gui, plus the two tiny
 * primitives the shared layer does not carry (a Medusa-shaped RadioGroup and
 * useToggleState). Everything here is a client component; the server-safe
 * pieces live in server.tsx.
 */
import * as React from "react"

import {
  Button as UIButton,
  type ButtonProps as UIButtonProps,
  type ButtonSize,
  type ButtonVariant,
} from "@hanzo/ui"

import { clx } from "./server"

export { Checkbox, Toaster, toast } from "@hanzo/ui"

/* -- Button ------------------------------------------------------------------
   Medusa's variant/size vocabulary, drawn by the shared @hanzo/ui Button. */

const BUTTON_VARIANT: Record<string, ButtonVariant> = {
  primary: "default",
  secondary: "secondary",
  transparent: "ghost",
  danger: "destructive",
}

const BUTTON_SIZE: Record<string, ButtonSize> = {
  small: "sm",
  base: "default",
  large: "lg",
  xlarge: "lg",
}

export interface ButtonProps extends Omit<UIButtonProps, "variant" | "size"> {
  variant?: "primary" | "secondary" | "transparent" | "danger"
  size?: "small" | "base" | "large" | "xlarge"
}

export const Button = React.forwardRef<
  React.ComponentRef<typeof UIButton>,
  ButtonProps
>(({ variant = "primary", size = "base", ...props }, ref) => (
  <UIButton
    ref={ref}
    variant={BUTTON_VARIANT[variant]}
    size={BUTTON_SIZE[size]}
    {...props}
  />
))
Button.displayName = "Button"

export interface IconButtonProps extends Omit<UIButtonProps, "variant" | "size"> {
  variant?: "primary" | "transparent"
  size?: "small" | "base" | "large" | "xlarge"
}

const ICON_SIZE: Record<string, ButtonSize> = {
  small: "icon-sm",
  base: "icon",
  large: "icon-lg",
  xlarge: "icon-lg",
}

export const IconButton = React.forwardRef<
  React.ComponentRef<typeof UIButton>,
  IconButtonProps
>(({ variant = "transparent", size = "base", ...props }, ref) => (
  <UIButton
    ref={ref}
    variant={variant === "primary" ? "default" : "ghost"}
    size={ICON_SIZE[size]}
    {...props}
  />
))
IconButton.displayName = "IconButton"

/* -- RadioGroup -----------------------------------------------------------------
   One group primitive, two item shapes: `Item` is a native radio bound to the
   group (Medusa's shape); `Option` is a selectable card (what the checkout
   draws payment/shipping choices with). Native inputs give the a11y. */

type RadioGroupContextValue = {
  value?: string | null
  setValue: (v: string) => void
  name: string
}

const RadioGroupContext = React.createContext<RadioGroupContextValue | null>(null)

const useRadioGroup = () => {
  const ctx = React.useContext(RadioGroupContext)
  if (!ctx) throw new Error("RadioGroup.Item must render inside RadioGroup")
  return ctx
}

export interface RadioGroupProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "onChange"> {
  value?: string | null
  onValueChange?: (value: string) => void
  name?: string
}

const RadioGroupRoot = React.forwardRef<HTMLDivElement, RadioGroupProps>(
  ({ value, onValueChange, name, children, ...props }, ref) => {
    const autoName = React.useId()
    const ctx = React.useMemo(
      () => ({
        value,
        setValue: (v: string) => onValueChange?.(v),
        name: name ?? autoName,
      }),
      [value, onValueChange, name, autoName]
    )
    return (
      <div ref={ref} role="radiogroup" {...props}>
        <RadioGroupContext.Provider value={ctx}>
          {children}
        </RadioGroupContext.Provider>
      </div>
    )
  }
)
RadioGroupRoot.displayName = "RadioGroup"

interface RadioGroupItemProps
  extends Omit<React.ComponentPropsWithoutRef<"input">, "type" | "value"> {
  value: string
}

const RadioGroupItem = React.forwardRef<HTMLInputElement, RadioGroupItemProps>(
  ({ className, value, checked, onChange, ...props }, ref) => {
    const group = useRadioGroup()
    return (
      <input
        ref={ref}
        type="radio"
        name={group.name}
        value={value}
        checked={checked ?? group.value === value}
        onChange={(e) => {
          onChange?.(e)
          if (e.target.checked) group.setValue(value)
        }}
        className={clx("kit-radio-item", className)}
        {...props}
      />
    )
  }
)
RadioGroupItem.displayName = "RadioGroup.Item"

interface RadioGroupOptionProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "onChange"> {
  value: string
  disabled?: boolean
}

const RadioGroupOption = React.forwardRef<HTMLDivElement, RadioGroupOptionProps>(
  ({ value, disabled, className, onClick, onKeyDown, ...props }, ref) => {
    const group = useRadioGroup()
    const checked = group.value === value
    return (
      <div
        ref={ref}
        role="radio"
        aria-checked={checked}
        aria-disabled={disabled || undefined}
        tabIndex={disabled ? -1 : 0}
        data-state={checked ? "checked" : "unchecked"}
        className={className}
        onClick={(e) => {
          onClick?.(e)
          if (!disabled) group.setValue(value)
        }}
        onKeyDown={(e) => {
          onKeyDown?.(e)
          if (disabled) return
          if (e.key === " " || e.key === "Enter") {
            e.preventDefault()
            group.setValue(value)
          }
        }}
        {...props}
      />
    )
  }
)
RadioGroupOption.displayName = "RadioGroup.Option"

export const RadioGroup = Object.assign(RadioGroupRoot, {
  Item: RadioGroupItem,
  Option: RadioGroupOption,
})

/* -- useToggleState ------------------------------------------------------------- */

export type ToggleState = {
  state: boolean
  open: () => void
  close: () => void
  toggle: () => void
}

export function useToggleState(initial = false): ToggleState {
  const [state, setState] = React.useState(initial)
  return {
    state,
    open: React.useCallback(() => setState(true), []),
    close: React.useCallback(() => setState(false), []),
    toggle: React.useCallback(() => setState((s) => !s), []),
  }
}
