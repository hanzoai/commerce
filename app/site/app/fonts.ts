import { Geist, Geist_Mono } from "next/font/google"

// The canonical Hanzo faces — Geist Sans for UI/body/headings, Geist Mono for
// code and data. The same pair the Commerce admin and the console load, bound
// to the variable names `@hanzo/ui/theme.css` reads, so the CSS-rendered MDX
// prose and the Gui-rendered components typeset identically.
export const sans = Geist({
  subsets: ["latin"],
  variable: "--font-geist-sans",
  display: "swap",
})

export const mono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
  display: "swap",
})
