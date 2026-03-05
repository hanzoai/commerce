import type { Metadata } from 'next'
import '@hanzo/commerce-ui/main.css'
import './globals.css'
import { Providers } from './providers'
import { ChatWidget } from '@/components/chat-widget'

export const metadata: Metadata = {
  title: 'Hanzo Commerce',
  description: 'AI-powered commerce platform by Hanzo',
  openGraph: {
    title: 'Hanzo Commerce',
    description: 'AI-powered commerce platform for modern businesses',
    url: 'https://commerce.hanzo.ai',
    siteName: 'Hanzo Commerce',
    type: 'website',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Hanzo Commerce',
    description: 'AI-powered commerce platform for modern businesses',
    creator: '@hanzoai',
  },
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
