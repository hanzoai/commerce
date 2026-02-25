import type { MetadataRoute } from "next"

const baseUrl =
  process.env.NEXT_PUBLIC_BASE_URL || "https://commerce.hanzo.ai"

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: baseUrl, lastModified: new Date() },
    { url: `${baseUrl}/auth/callback`, lastModified: new Date() },
  ]
}
