'use client'

import type { ReactNode } from 'react'
import { YStack } from '@hanzo/gui'

export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <YStack minH="100vh" items="center" justify="center" bg="$background" p="$4">
      {children}
    </YStack>
  )
}
