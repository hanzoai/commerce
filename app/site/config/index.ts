const baseUrl = process.env.NEXT_PUBLIC_BASE_URL || "http://localhost:3002"

export const config = {
  titleSuffix: "Hanzo Commerce Documentation",
  description:
    "Explore and learn how to use Hanzo Commerce. AI-powered commerce infrastructure for modern businesses.",
  baseUrl,
  basePath: process.env.NEXT_PUBLIC_BASE_PATH,
  project: {
    title: "Documentation",
    key: "book",
  },
  logo: `/images/logo.png`,
  breadcrumbOptions: {
    startItems: [
      {
        title: "Documentation",
        link: "/",
      },
    ],
  },
}
