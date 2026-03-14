import insights from "@hanzo/insights"

insights.init(
  (process.env.NEXT_PUBLIC_INSIGHTS_KEY || process.env.NEXT_PUBLIC_INSIGHTS_KEY)!,
  {
    api_host:
      process.env.NEXT_PUBLIC_INSIGHTS_HOST ||
      process.env.NEXT_PUBLIC_INSIGHTS_HOST,
    person_profiles: "always",
    defaults: "2025-05-24",
  }
)
