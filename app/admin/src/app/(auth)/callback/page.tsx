'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Card, Spinner, Text, YStack } from '@hanzo/gui'

const IAM_SERVER = process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'
const CLIENT_ID = process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce'

export default function CallbackPage() {
  const router = useRouter()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void import('@hanzo/iam/browser').then(({ IAM }) =>
      new IAM({
        serverUrl: IAM_SERVER,
        clientId: CLIENT_ID,
        redirectUri: `${window.location.origin}/callback`,
      })
        .handleCallback(window.location.href)
        // Full page load so IamProvider picks up the stored tokens.
        .then(() => window.location.assign('/overview'))
        .catch((e: Error) => setError(e.message || 'Authentication failed.')),
    )
  }, [])

  if (error) {
    return (
      <Card maxWidth={420} p="$5" gap="$3" borderWidth={1} borderColor="$borderColor">
        <Text fontSize="$5" fontWeight="500">
          Sign in failed
        </Text>
        <Text fontSize="$3" color="$color11">
          {error}
        </Text>
        <Text fontSize="$3" color="$color11" cursor="pointer" onPress={() => router.replace('/login')}>
          Back to sign in
        </Text>
      </Card>
    )
  }

  return (
    <YStack items="center" gap="$3">
      <Spinner />
      <Text fontSize="$3" color="$color11">
        Signing in…
      </Text>
    </YStack>
  )
}
