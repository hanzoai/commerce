"use client"

/**
 * Root client providers — the Hanzo GUI theme context, and nothing else.
 * The scale is `@hanzo/ui/gui-config`, THE shared type/radius/spacing scale
 * the Commerce admin, the docs site and the console all render on. The
 * storefront is a light surface (html.light + data-mode="light").
 */
import type { ReactNode } from "react"
import { GuiProvider } from "@hanzo/gui"
import config from "@hanzo/ui/gui-config"

const Providers = ({ children }: { children?: ReactNode }) => (
  <GuiProvider config={config} defaultTheme="light">
    {children}
  </GuiProvider>
)

export default Providers
