import type { Metadata } from "next"
import { inter, robotoMono } from "./fonts"
import "./globals.css"

export const metadata: Metadata = {
  title: "Hanzo Commerce - AI-Powered Commerce Platform",
  description:
    "Build, launch, and scale your commerce business with AI-powered tools. Headless APIs, multi-currency support, intelligent pricing, and real-time analytics.",
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_BASE_URL || "https://commerce.hanzo.ai"
  ),
  openGraph: {
    siteName: "Hanzo Commerce",
    type: "website",
    title: "Hanzo Commerce - AI-Powered Commerce Platform",
    description:
      "Build, launch, and scale your commerce business with AI-powered tools.",
  },
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${robotoMono.variable} dark`}
      suppressHydrationWarning
    >
      <body className="font-sans">{children}</body>
    </html>
  )
}
