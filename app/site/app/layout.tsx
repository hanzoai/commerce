import type { Metadata } from "next"
import "./globals.css"
import { config } from "@/config"
import { inter, robotoMono } from "./fonts"
import clsx from "clsx"

export const metadata: Metadata = {
  title: {
    template: `%s - ${config.titleSuffix}`,
    default: config.titleSuffix || "",
  },
  description: config.description,
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_BASE_URL || "http://localhost:3002"
  ),
  openGraph: {
    siteName: "Hanzo Commerce",
    type: "website",
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
      className={clsx(inter.variable, robotoMono.variable)}
      suppressHydrationWarning
    >
      {children}
    </html>
  )
}
