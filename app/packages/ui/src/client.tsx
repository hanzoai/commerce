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
  DropdownMenu as UIDropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Tabs as UITabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@hanzo/ui"

import { clx } from "./server"

export { Switch, Toaster, toast } from "@hanzo/ui"

/* -- Checkbox --------------------------------------------------------------------
   A button carrying `role="checkbox"`, which is what the storefront's checkout
   form already talks to: it holds the checked value itself and listens on
   `onClick`. @hanzo/ui's Checkbox owns its own state and speaks the Gui prop
   vocabulary instead, so it cannot take that call. Native semantics, same as
   RadioGroup below. */

export type CheckedState = boolean | "indeterminate"

export interface CheckboxProps
  extends Omit<React.ComponentPropsWithoutRef<"button">, "checked" | "onChange"> {
  checked?: CheckedState
  onCheckedChange?: (checked: CheckedState) => void
}

export const Checkbox = React.forwardRef<HTMLButtonElement, CheckboxProps>(
  ({ className, checked = false, onCheckedChange, onClick, ...props }, ref) => {
    const mixed = checked === "indeterminate"
    return (
      <button
        {...props}
        ref={ref}
        type="button"
        role="checkbox"
        aria-checked={mixed ? "mixed" : checked}
        data-state={mixed ? "indeterminate" : checked ? "checked" : "unchecked"}
        className={clx("kit-checkbox", className)}
        onClick={(e) => {
          onClick?.(e)
          onCheckedChange?.(mixed ? true : !checked)
        }}
      />
    )
  }
)
Checkbox.displayName = "Checkbox"

/* -- DropdownMenu, Tabs --------------------------------------------------------
   @hanzo/ui spells these flat (DropdownMenuTrigger); the kit's call sites reach
   for them through the root (DropdownMenu.Trigger), the same compound shape
   Table and RadioGroup take below. */

export const DropdownMenu = Object.assign(UIDropdownMenu, {
  Trigger: DropdownMenuTrigger,
  Content: DropdownMenuContent,
  Item: DropdownMenuItem,
  Label: DropdownMenuLabel,
  Separator: DropdownMenuSeparator,
})

export const Tabs = Object.assign(UITabs, {
  List: TabsList,
  Trigger: TabsTrigger,
  Content: TabsContent,
})

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

/* -- useToggleState -------------------------------------------------------------
   Both a tuple and a record, so a call site may destructure either way:

     const [open, show, hide, toggle] = useToggleState()
     const { state, open, close, toggle } = useToggleState()

   The storefront reads it both ways and passes the value on to components that
   name the tuple half in their props, so the shape is the contract. */

export type ToggleState = [boolean, () => void, () => void, () => void] & {
  state: boolean
  open: () => void
  close: () => void
  toggle: () => void
}

export function useToggleState(initial = false): ToggleState {
  const [state, setState] = React.useState(initial)

  const open = React.useCallback(() => setState(true), [])
  const close = React.useCallback(() => setState(false), [])
  const toggle = React.useCallback(() => setState((s) => !s), [])

  return React.useMemo(() => {
    const value = [state, open, close, toggle] as ToggleState
    value.state = state
    value.open = open
    value.close = close
    value.toggle = toggle
    return value
  }, [state, open, close, toggle])
}
