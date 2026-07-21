"use client"

import { ReactNode, useEffect, useMemo } from "react"
import { usePathname } from "next/navigation"
import { createAnalytics } from "@hanzo/event"
import { AnalyticsProvider, useAnalytics, usePageview } from "@hanzo/event/react"

/** Cloud analytics ingest — api.hanzo.ai fronts /v1/analytics (+ /v1/tracker). */
const HOST = "https://api.hanzo.ai"

function Pageview() {
  usePageview(usePathname())
  return null
}

/**
 * AnalyticsIdentify binds the current visitor to a stable person id once the
 * storefront's Medusa customer resolves. Mount it wherever the customer is
 * already fetched (e.g. the (main) layout) so it costs no extra request. It
 * reads the client from the AnalyticsRoot context above it.
 */
export function AnalyticsIdentify({ personId }: { personId?: string | null }) {
  const analytics = useAnalytics()
  useEffect(() => {
    if (personId) analytics.identify(personId)
  }, [personId, analytics])
  return null
}

/**
 * Telemetry root for the commerce storefront. Wraps the app in the ONE shared
 * @hanzo/event client (product='commerce') and emits pageviews on every route
 * change. The storefront's bearer is an httpOnly Medusa cookie the browser
 * cannot read, so ingest is anonymous (getToken omitted) — identify() still
 * binds the person id via AnalyticsIdentify once auth resolves. Purely
 * additive: on any failure the client no-ops and never touches UX.
 */
export function AnalyticsRoot({ children }: { children: ReactNode }) {
  const client = useMemo(
    () => createAnalytics({ product: "commerce", host: HOST }),
    []
  )

  return (
    <AnalyticsProvider client={client}>
      <Pageview />
      {children}
    </AnalyticsProvider>
  )
}
