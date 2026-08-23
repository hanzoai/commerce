"use client"

/**
 * Root client providers — the Hanzo GUI theme, and nothing else.
 *
 * The docs are a static export with no session and no data layer, so the only
 * thing every page needs is the token/theme context the `@hanzo/gui` components
 * and the `@hanzo/ui` component layer read from. The scale is
 * `@hanzo/ui/gui-config` — the same one the Commerce admin renders on.
 */
import type { ReactNode } from "react"
import { GuiProvider } from "@hanzo/gui"
import config from "@hanzo/ui/gui-config"

const Providers = ({ children }: { children?: ReactNode }) => (
  <GuiProvider config={config} defaultTheme="dark">
    {children}
  </GuiProvider>
)

export default Providers
