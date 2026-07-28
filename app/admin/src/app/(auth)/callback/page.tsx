'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Card, Spinner, Text, YStack } from '@hanzo/gui'

import { BASE_PATH } from '@/lib/basepath'
import { iamConfig } from '@/lib/iam'

export default function CallbackPage() {
  const router = useRouter()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    void import('@hanzo/iam/browser').then(({ IAM }) =>
      new IAM(iamConfig())
        .handleCallback(window.location.href)
        // Full page load so IamProvider picks up the stored tokens — a hard
        // navigation, so it spells the mount point the router would have added.
        .then(() => window.location.assign(`${BASE_PATH}/overview`))
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
