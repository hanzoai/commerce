import type { Metadata } from 'next'
import '@hanzo/commerce-ui/main.css'
import './globals.css'
import { Providers } from './providers'

export const metadata: Metadata = {
  title: 'Hanzo Commerce Dashboard',
  description: 'Admin dashboard for Hanzo Commerce',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className="dark">
      <body className="font-sans">
        <Providers>
          {children}
        </Providers>
      </body>
    </html>
  )
}
