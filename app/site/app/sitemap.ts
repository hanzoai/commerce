import type { MetadataRoute } from "next"

const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || "https://commerce.hanzo.ai"

export default function sitemap(): MetadataRoute.Sitemap {
  return [
    { url: baseUrl },
    { url: `${baseUrl}/learn` },
    { url: `${baseUrl}/learn/installation` },
    { url: `${baseUrl}/learn/fundamentals/framework` },
    { url: `${baseUrl}/learn/fundamentals/modules` },
    { url: `${baseUrl}/learn/fundamentals/workflows` },
    { url: `${baseUrl}/learn/fundamentals/api-routes` },
    { url: `${baseUrl}/learn/fundamentals/data-models` },
    { url: `${baseUrl}/learn/fundamentals/admin` },
    { url: `${baseUrl}/learn/deployment` },
  ]
}
