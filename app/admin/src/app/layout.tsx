import type { Metadata } from 'next'
import '../index.css'
import { Providers } from './providers'

export const metadata: Metadata = {
  title: 'Hanzo Commerce',
  description: 'Build and run your store with Hanzo Commerce',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  )
}
