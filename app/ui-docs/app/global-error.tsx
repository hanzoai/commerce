"use client"

import { BareboneLayout, BrowserProvider, ErrorPage } from "@hanzo/commerce-docs-ui"
import { inter, robotoMono } from "./fonts"
import clsx from "clsx"
import "./globals.css"

export default function Error() {
  return (
    <BareboneLayout
      htmlClassName={clsx(inter.variable, robotoMono.variable)}
      gaId={process.env.NEXT_PUBLIC_GA_ID}
    >
      <body className="w-screen h-screen overflow-hidden bg-hanzo-bg-subtle">
        <BrowserProvider>
          <ErrorPage />
        </BrowserProvider>
      </body>
    </BareboneLayout>
  )
}
