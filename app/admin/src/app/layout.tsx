import type { Metadata } from 'next'
import '../index.css'
import { Providers } from './providers'

export const metadata: Metadata = {
  title: 'Hanzo Commerce',
  description: 'Hanzo Commerce Admin Dashboard',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
