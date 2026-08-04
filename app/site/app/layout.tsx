import type { Metadata } from "next"
import "./globals.css"
import { config } from "@/config"
import { sans, mono } from "./fonts"

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

// `t_dark` is the Gui theme class; `dark` is what @hanzo/ui/theme.css keys the
// dark token set on. Both, once, on the element every page inherits from.
export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html
      lang="en"
      className={`${sans.variable} ${mono.variable} dark t_dark`}
      suppressHydrationWarning
    >
      {children}
    </html>
  )
}
