'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Card, Text, YStack } from '@hanzo/gui'
import { HanzoMark, PrimaryButton } from '@hanzo/ui/product'

const IAM_SERVER = process.env.NEXT_PUBLIC_IAM_SERVER_URL || 'https://hanzo.id'
const CLIENT_ID = process.env.NEXT_PUBLIC_IAM_CLIENT_ID || 'hanzo-commerce'

export default function LoginPage() {
  const router = useRouter()

  useEffect(() => {
    if (sessionStorage.getItem('hanzo_iam_access_token')) router.replace('/overview')
  }, [router])

  // Dynamic import — the SDK touches Web Crypto, which the export prerender lacks.
  const signIn = async () => {
    const { IAM } = await import('@hanzo/iam/browser')
    await new IAM({
      serverUrl: IAM_SERVER,
      clientId: CLIENT_ID,
      redirectUri: `${window.location.origin}/callback`,
    }).signinRedirect()
  }

  return (
    <Card width="100%" maxWidth={360} p="$6" gap="$4" borderWidth={1} borderColor="$borderColor" items="center">
      <HanzoMark size={40} />
      <YStack gap="$1" items="center">
        <Text fontSize="$6" fontWeight="500">
          Hanzo Commerce
        </Text>
        <Text fontSize="$3" color="$color11">
          Sign in to build and run your store
        </Text>
      </YStack>
      <PrimaryButton width="100%" onPress={() => void signIn()}>
        Sign in with Hanzo ID
      </PrimaryButton>
      <Text fontSize="$2" color="$color10" text="center">
        You will be redirected to {new URL(IAM_SERVER).host} to authenticate via OIDC.
      </Text>
    </Card>
  )
}
