/**
 * Registers THE shared Gui scale with the type system so shorthand style props
 * (tokens, themes, bg/px/py/items/justify etc.) are typed correctly.
 *
 * The config is `@hanzo/ui/gui-config` — the one type, radius and spacing scale
 * that ships WITH the components this admin renders. A local copy is a fork: the
 * shared @hanzo/ui/product components are TYPED against that scale, so a private
 * `$5` radius here means they draw at a size their author never chose.
 *
 * GuiCustomConfig is declared in @hanzogui/web and flows through @hanzo/gui.
 */
import type { Conf } from '@hanzo/ui/gui-config'

declare module '@hanzogui/web' {
  interface GuiCustomConfig extends Conf {}
}

declare module '@hanzogui/core' {
  interface GuiCustomConfig extends Conf {}
}
