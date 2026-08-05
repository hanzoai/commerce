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
import { useRouter } from 'next/navigation'
import { GuiProvider } from '@hanzo/gui'
import { IamProvider } from '@hanzo/iam/react'
import { HostProvider } from '@hanzo/ui/product'

import config from '../../gui.config'
import { BASE_PATH } from '@/lib/basepath'
import { iamConfig } from '@/lib/iam'

export function Providers({ children }: { children: ReactNode }) {
  const router = useRouter()
  // IamProvider reads sessionStorage, which does not exist during the export
  // prerender. Mount after hydration; children that call useIam() need it present.
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])
  if (!mounted) return null

  return (
    <GuiProvider config={config} defaultTheme="dark">
      <IamProvider config={iamConfig()}>
        <HostProvider
          actions={{
            // A route of this export — the router applies basePath.
            signIn: () => router.push('/login'),
            // NOT a route of this export: `<base>/billing` is the hanzoai/billing
            // bundle the Go binary mounts alongside us, so this is a real
            // navigation and has to spell the mount point.
            addCredits: () => window.location.assign(`${BASE_PATH}/billing`),
          }}
        >
          {children}
        </HostProvider>
      </IamProvider>
    </GuiProvider>
  )
}
