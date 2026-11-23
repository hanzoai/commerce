'use client'

/**
 * Root client providers — Hanzo GUI theme + IAM session + multi-org context +
 * the `@hanzo/ui/product` host seam.
 */
import { useEffect, useState, type ReactNode } from 'react'
import { useRouter } from 'next/navigation'
import { GuiProvider } from '@hanzo/gui'
import { IamProvider } from '@hanzo/iam/react'
import { HostProvider } from '@hanzo/ui/product'

import config from '../../gui.config'
import { iamConfig } from '@/lib/iam'
import { ActiveOrgProvider } from '@/lib/org'

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
        <ActiveOrgProvider>
          <HostProvider
            actions={{
              signIn: () => router.push('/login'),
              addCredits: () => window.location.assign('https://hanzo.ai/pay'),
            }}
          >
            {children}
          </HostProvider>
        </ActiveOrgProvider>
      </IamProvider>
    </GuiProvider>
  )
}
