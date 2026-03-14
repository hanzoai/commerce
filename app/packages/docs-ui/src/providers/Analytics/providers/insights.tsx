import insights from "posthog-js"
import { TrackedEvent } from ".."

export const useInsightsAnalytics = () => {
  const track = async ({ event, options }: TrackedEvent) => {
    insights.capture(event, options)
  }

  return {
    track,
  }
}
