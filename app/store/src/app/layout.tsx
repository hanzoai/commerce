import { getBaseURL } from "@lib/util/env"
import { Metadata } from "next"
import "../styles/globals.css"
import { AnalyticsRoot } from "@modules/analytics"

export const metadata: Metadata = {
  metadataBase: new URL(getBaseURL()),
}

export default function RootLayout(props: { children: React.ReactNode }) {
  return (
    <html lang="en" data-mode="light">
      <body>
        <AnalyticsRoot>
          <main className="relative">{props.children}</main>
        </AnalyticsRoot>
      </body>
    </html>
  )
}
