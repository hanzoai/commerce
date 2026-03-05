'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Heading, Text } from '@hanzo/commerce-ui'

const IAM_SERVER = process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'
const CLIENT_ID = process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce-client-id'

export default function CallbackPage() {
  const router = useRouter()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // Dynamic import to avoid SSR issues with Web Crypto APIs
    import('@hanzo/iam/browser').then(({ BrowserIamSdk }) => {
      const sdk = new BrowserIamSdk({
        serverUrl: IAM_SERVER,
        clientId: CLIENT_ID,
        redirectUri: `${window.location.origin}/callback`,
      })
      sdk.handleCallback(window.location.href)
        .then(() => {
          // Full page load so IamProvider picks up stored tokens
          window.location.href = '/overview'
        })
        .catch((err) => {
          setError(err.message || 'Authentication failed.')
        })
    })
  }, [])

  if (error) {
    return (
      <div className="text-center">
        <Heading level="h2">Sign In Failed</Heading>
        <Text size="small" className="mt-2 text-ui-fg-subtle">{error}</Text>
        <Button
          variant="secondary"
          className="mt-4"
          onClick={() => router.replace('/login')}
        >
          Back to Login
        </Button>
      </div>
    )
  }

  return (
    <div className="text-center">
      <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-2 border-white border-t-transparent" />
      <Text size="small" className="text-ui-fg-muted">Signing in...</Text>
    </div>
  )
}
