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
    // theme.css (8.0.47) is dark-FIRST: `:root` is the dark look and `.light`
    // is the defined opt-out — the storefront is a light surface, so it opts
    // out. data-mode is the storefront's own light palette hook. One surface,
    // one mode. (At 8.0.39 the convention was inverted and `.light` had no
    // rule at all; gui-css-check is what notices either way.)
    <html lang="en" data-mode="light" className="light">
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
