'use client'

import { useState, useEffect } from 'react'
import { IamProvider } from '@hanzo/iam/react'
import { QueryProvider } from '@/lib/query-provider'

const IAM_CONFIG = {
  serverUrl: process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id',
  clientId: process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce-client-id',
  redirectUri: typeof window !== 'undefined' ? `${window.location.origin}/callback` : '',
}

export function Providers({ children }: { children: React.ReactNode }) {
  // IamProvider accesses sessionStorage which doesn't exist during SSR prerender.
  // Only mount after hydration. Children that call useIam() need the provider present.
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])

  if (!mounted) {
    return null
  }

  return (
    <QueryProvider>
      <IamProvider config={IAM_CONFIG}>
        {children}
      </IamProvider>
    </QueryProvider>
  )
}
