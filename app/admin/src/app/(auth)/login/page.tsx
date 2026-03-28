'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Heading, Text } from '@hanzo/commerce-ui'

const IAM_SERVER = process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'
const CLIENT_ID = process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce'

export default function LoginPage() {
  const router = useRouter()

  useEffect(() => {
    // If already authenticated, redirect to dashboard
    const token = sessionStorage.getItem('hanzo_iam_access_token')
    if (token) router.replace('/overview')
  }, [router])

  const handleLogin = async () => {
    // Dynamic import to avoid SSR issues with Web Crypto APIs
    const { BrowserIamSdk } = await import('@hanzo/iam/browser')
    const sdk = new BrowserIamSdk({
      serverUrl: IAM_SERVER,
      clientId: CLIENT_ID,
      redirectUri: `${window.location.origin}/callback`,
    })
    await sdk.signinRedirect()
  }

  return (
    <div className="w-full max-w-sm">
      <div className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-8">
        <div className="flex flex-col items-center text-center">
          <svg viewBox="0 0 67 67" className="mb-6 h-12 w-12 text-white" xmlns="http://www.w3.org/2000/svg">
            <path d="M22.21 67V44.6369H0V67H22.21Z" fill="currentColor"/>
            <path d="M0 44.6369L22.21 46.8285V44.6369H0Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7038 22.3184H22.2534L0.0878906 44.6367H44.4634L66.7038 22.3184Z" fill="currentColor"/>
            <path d="M22.21 0H0V22.3184H22.21V0Z" fill="currentColor"/>
            <path d="M66.7198 0H44.5098V22.3184H66.7198V0Z" fill="currentColor"/>
            <path d="M66.6753 22.3185L44.5098 20.0822V22.3185H66.6753Z" fill="currentColor" opacity="0.7"/>
            <path d="M66.7198 67V44.6369H44.5098V67H66.7198Z" fill="currentColor"/>
          </svg>

          <Heading level="h1">Hanzo Commerce</Heading>
          <Text size="small" className="mt-1 text-ui-fg-subtle">
            Sign in to access the admin dashboard
          </Text>

          <div className="mt-8 w-full space-y-3">
            <Button
              className="w-full"
              onClick={handleLogin}
            >
              Sign in with Hanzo ID
            </Button>

            <Text size="xsmall" className="text-ui-fg-muted">
              You will be redirected to{' '}
              <span className="text-ui-fg-base">hanzo.id</span>{' '}
              to authenticate via OIDC
            </Text>
          </div>
        </div>
      </div>
    </div>
  )
}
