'use client'

// One titled card used for the sub-resource + preview sections on a resource
// detail page (a promotion's application method, a price list's prices, a
// region's countries, the calculate/evaluate previews). Keeps the section
// chrome — bordered container, title/description header, optional action — in
// exactly one place so no detail view re-implements it.

import type { ReactNode } from 'react'
import { Container, Heading, Text } from '@hanzo/commerce-ui'

interface DetailPanelProps {
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
}

export function DetailPanel({ title, description, action, children }: DetailPanelProps) {
  return (
    <Container className="flex w-full flex-col gap-y-4 p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Heading level="h2">{title}</Heading>
          {description && (
            <Text size="small" leading="compact" className="mt-1 text-ui-fg-subtle">
              {description}
            </Text>
          )}
        </div>
        {action && <div className="flex items-center gap-2">{action}</div>}
      </div>
      {children}
    </Container>
  )
}
