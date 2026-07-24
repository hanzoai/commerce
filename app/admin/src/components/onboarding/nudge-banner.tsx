'use client'

import Link from 'next/link'
import { Badge, Button, Container, Heading, Text } from '@hanzo/commerce-ui'
import { useStore } from '@/lib/api/hooks'

// First-run nudge the overview page renders when the org has no store yet.
// Self-gating: returns null once a store exists, so the overview page can render
// it unconditionally without duplicating the store check.
export function OnboardingNudge() {
  const { data: store, isLoading } = useStore()
  if (isLoading || store) return null

  return (
    <Container className="mb-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <Heading level="h2">Set up your store</Heading>
            <Badge color="orange">Get started</Badge>
          </div>
          <Text size="small" className="mt-1 text-ui-fg-muted">
            Create a store, add a product, connect payments, and start your trial — the guided setup takes a couple of minutes.
          </Text>
        </div>
        <Link href="/onboarding">
          <Button variant="primary" size="small">Start setup</Button>
        </Link>
      </div>
    </Container>
  )
}
