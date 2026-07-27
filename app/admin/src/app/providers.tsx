'use client'

/**
 * Root client providers — Hanzo GUI theme + IAM session + the `@hanzo/ui/product`
 * host seam.
 *
 * The product layer is presentational and never imports a router or an auth
 * module, so the two effects its honest-state cards can offer (re-auth on 401,
 * top-up on 402) are injected here, once.
 */
import { useEffect, useState, type ReactNode } from 'react'
import { GuiProvider } from '@hanzo/gui'
import { IamProvider } from '@hanzo/iam/react'
import { HostProvider } from '@hanzo/ui/product'

import config from '../../gui.config'

const IAM_CONFIG = {
  serverUrl: process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id',
  clientId: process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce',
  redirectUri: typeof window !== 'undefined' ? `${window.location.origin}/callback` : '',
}

export function Providers({ children }: { children: ReactNode }) {
  // IamProvider reads sessionStorage, which does not exist during the export
  // prerender. Mount after hydration; children that call useIam() need it present.
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])
  if (!mounted) return null

  return (
    <GuiProvider config={config} defaultTheme="dark">
      <IamProvider config={IAM_CONFIG}>
        <HostProvider
          actions={{
            signIn: () => window.location.assign('/login'),
            addCredits: () => window.location.assign('/billing'),
          }}
        >
          {children}
        </HostProvider>
      </IamProvider>
    </GuiProvider>
  )
}
