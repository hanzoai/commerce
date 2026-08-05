/**
 * Registers THE shared Gui scale with the type system so shorthand style props
 * (tokens, themes, bg/px/py/items/justify etc.) are typed correctly.
 *
 * The config is `@hanzo/ui/gui-config` — the one type, radius and spacing scale
 * that ships WITH the components, so the storefront cannot drift from the
 * Commerce admin, the docs site or the console. A local copy would be a fork.
 */
import type { Conf } from "@hanzo/ui/gui-config"

declare module "@hanzogui/web" {
  interface GuiCustomConfig extends Conf {}
}

declare module "@hanzogui/core" {
  interface GuiCustomConfig extends Conf {}
}
