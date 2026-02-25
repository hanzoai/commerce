"use client"

import React from "react"
import { useInstantSearch } from "react-instantsearch"
import { useHanzoSuggestions } from "../../../hooks/use-hanzo-suggestions"
import { Card } from "../../Card"
import { useAnalytics } from "../../../providers/Analytics"
import { DocsTrackingEvents } from "../../../constants"

export const SearchCallout = () => {
  const { results } = useInstantSearch()
  const query = results?.query || ""
  const matchedCallout = useHanzoSuggestions({ keywords: query })
  const { track } = useAnalytics()

  if (!matchedCallout) {
    return null
  }

  return (
    <div className="m-docs_1 flex justify-center items-center">
      <Card
        {...matchedCallout}
        type="bloom"
        onClick={() => {
          track({
            event: {
              event: DocsTrackingEvents.SEARCH_CALLOUT_CLICK,
              options: {
                user_keywords: query,
                callout_title: matchedCallout.title,
                callout_href: matchedCallout.href,
              },
            },
          })
        }}
      />
    </div>
  )
}
