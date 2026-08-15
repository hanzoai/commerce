import { getBaseURL } from "@lib/util/env"
import { Metadata } from "next"
// One sheet: globals.css pulls in the shared tokens and the kit around the
// Tailwind layers, in the order the cascade needs them.
import "../styles/globals.css"
import { AnalyticsRoot } from "@modules/analytics"
import Providers from "./providers"

export const metadata: Metadata = {
  metadataBase: new URL(getBaseURL()),
}

export default function RootLayout(props: { children: React.ReactNode }) {
  return (
    // theme.css is dark-first: `:root` carries the dark look and `.light` is
    // the defined opt-out. The storefront is a light surface, so it opts out.
    // data-mode is the storefront's own light palette hook.
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
