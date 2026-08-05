import { getBaseURL } from "@lib/util/env"
import { Metadata } from "next"
// Token order matters: theme.css declares the custom properties, kit.css
// draws the shared kit on them, and the utility stylesheet comes LAST so a
// caller's className can still override a kit default while it exists.
import "@hanzo/ui/theme.css"
import "@hanzo/commerce-ui/kit.css"
import "../styles/globals.css"
import { AnalyticsRoot } from "@modules/analytics"
import Providers from "./providers"

export const metadata: Metadata = {
  metadataBase: new URL(getBaseURL()),
}

export default function RootLayout(props: { children: React.ReactNode }) {
  return (
    // theme.css is light-first (`:root` is the light look, `.dark` the
    // opt-in), so no class is needed for light; data-mode is the storefront's
    // own light palette hook. One surface, one mode.
    <html lang="en" data-mode="light">
      <body>
        <Providers>
          <AnalyticsRoot>
            <main className="relative">{props.children}</main>
          </AnalyticsRoot>
        </Providers>
      </body>
    </html>
  )
}
