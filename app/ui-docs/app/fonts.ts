import localFont from "next/font/local"
import { Geist_Mono } from "next/font/google"

// Basel Grotesk (self-hosted). Kept exported as `inter` and bound to the
// `--font-inter` CSS variable so existing layout.tsx className bindings keep
// working — only the underlying face changed (Inter -> Basel).
export const inter = localFont({
  variable: "--font-inter",
  display: "swap",
  src: [
    {
      path: "./fonts/Basel-Grotesk-Book.woff2",
      weight: "400",
      style: "normal",
    },
    {
      path: "./fonts/Basel-Grotesk-Medium.woff2",
      weight: "500",
      style: "normal",
    },
  ],
})

// Geist Mono. Kept exported as `robotoMono` / `--font-roboto-mono` so existing
// bindings keep working — only the underlying face changed (Roboto Mono -> Geist Mono).
export const robotoMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-roboto-mono",
})
