"use client"

import { inter } from "./fonts"
import "./globals.css"

export default function GlobalError() {
  return (
    <html lang="en" className={`${inter.variable} dark`}>
      <body className="flex min-h-screen items-center justify-center bg-[#0a0a0a] font-sans">
        <div className="text-center">
          <h1 className="mb-4 text-4xl font-bold text-white">
            Something went wrong
          </h1>
          <p className="mb-8 text-gray-400">An unexpected error occurred.</p>
          <a
            href="/"
            className="rounded-lg bg-brand px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-brand-500"
          >
            Go Home
          </a>
        </div>
      </body>
    </html>
  )
}
