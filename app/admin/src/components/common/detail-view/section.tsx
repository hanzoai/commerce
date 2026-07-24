'use client'

// A titled card section for detail pages: a heading, an optional right-aligned
// action/badge, then its rows/children (use SectionRow for label→value rows).
import { Container, Heading } from '@hanzo/commerce-ui'

interface SectionProps {
  title: string
  action?: React.ReactNode
  children?: React.ReactNode
}

export function Section({ title, action, children }: SectionProps) {
  return (
    <Container className="divide-y p-0">
      <div className="flex items-center justify-between gap-2 px-6 py-4">
        <Heading level="h2">{title}</Heading>
        {action}
      </div>
      {children}
    </Container>
  )
}
