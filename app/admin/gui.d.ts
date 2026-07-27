/**
 * Registers our Gui config with the type system so shorthand style props
 * (tokens, themes, bg/px/py/items/justify etc.) are typed correctly.
 * GuiCustomConfig is declared in @hanzogui/web and flows through @hanzo/gui.
 */
import type { Conf } from './gui.config'

declare module '@hanzogui/web' {
  interface GuiCustomConfig extends Conf {}
}

declare module '@hanzogui/core' {
  interface GuiCustomConfig extends Conf {}
}
