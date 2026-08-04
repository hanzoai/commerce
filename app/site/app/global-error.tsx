"use client"

import { sans, mono } from "./fonts"
import "./globals.css"

/**
 * The last-resort shell. It renders its own <html> because it replaces the root
 * layout, and it deliberately uses plain elements and the tokens from
 * `globals.css` — a provider that failed to mount is exactly what got us here.
 */
export default function GlobalError() {
  return (
    <html lang="en" className={`${sans.variable} ${mono.variable}`}>
      <body>
        <main
          style={{
            minHeight: "100vh",
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            justifyContent: "center",
            gap: "1rem",
            padding: "1rem",
            textAlign: "center",
          }}
        >
          <h1 style={{ fontSize: "2rem", fontWeight: 500, margin: 0 }}>
            Something went wrong
          </h1>
          <p style={{ color: "var(--muted-foreground)", margin: 0 }}>
            An unexpected error occurred.
          </p>
          <a
            href="/"
            style={{
              padding: "0.5rem 1rem",
              borderRadius: "var(--radius)",
              background: "var(--foreground)",
              color: "var(--background)",
              fontSize: "0.875rem",
              fontWeight: 500,
              textDecoration: "none",
            }}
          >
            Go Home
          </a>
        </main>
      </body>
    </html>
  )
}
