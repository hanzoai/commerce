/**
 * Registers the Gui config with the type system, so shorthand style props
 * (tokens, themes, bg/px/py/items/justify …) carry real names rather than the
 * library's generic fallback, and the config this app hands GuiProvider is the
 * one the provider expects. GuiCustomConfig is declared in both @hanzogui/web
 * and @hanzogui/core; the admin registers the same pair.
 *
 * The scale is @hanzo/ui's — the one the admin, the docs site and the console
 * all render on.
 */
import type { Conf } from "@hanzo/ui/gui-config"

declare module "@hanzogui/web" {
  interface GuiCustomConfig extends Conf {}
}

declare module "@hanzogui/core" {
  interface GuiCustomConfig extends Conf {}
}
