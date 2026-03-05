import type { Metadata } from 'next'
import '@hanzo/commerce-ui/main.css'
import './globals.css'
import { Providers } from './providers'
import { ChatWidget } from '@/components/chat-widget'

export const metadata: Metadata = {
  title: 'Hanzo Commerce',
  description: 'AI-powered commerce platform by Hanzo',
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
          <ChatWidget />
        </Providers>
      </body>
    </html>
  )
}
