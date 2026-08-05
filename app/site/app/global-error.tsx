"use client"

import { inter, robotoMono } from "./fonts"
import clsx from "clsx"
import "./globals.css"

export default function Error() {
  return (
    <html lang="en" className={clsx(inter.variable, robotoMono.variable)}>
      <body className="w-screen h-screen overflow-hidden bg-gray-100 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-4xl font-bold mb-4">Something went wrong</h1>
          <p className="text-gray-500 mb-8">An unexpected error occurred.</p>
          <a
            href="/"
            className="px-4 py-2 bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-lg text-sm font-medium"
          >
            Go Home
          </a>
        </div>
      </body>
    </html>
  )
}
