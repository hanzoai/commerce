'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Heading, Text } from '@hanzo/commerce-ui'

const IAM_SERVER = process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'
const CLIENT_ID = process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce-client-id'

export default function LoginPage() {
  const router = useRouter()

  useEffect(() => {
    // If already authenticated, redirect to dashboard
    const token = sessionStorage.getItem('hanzo_iam_access_token')
    if (token) router.replace('/')
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
          <div className="mb-6 flex h-12 w-12 items-center justify-center rounded-lg bg-white">
            <span className="text-lg font-bold text-black">H</span>
          </div>

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
