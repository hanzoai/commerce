import posthog from "posthog-js"
import { TrackedEvent } from ".."

export const useInsightsAnalytics = () => {
  const track = async ({ event, options }: TrackedEvent) => {
    posthog.capture(event, options)
  }

  return {
    track,
  }
}
