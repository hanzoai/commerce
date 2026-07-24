import Link from 'next/link'
import { Heading, Text, Container } from '@hanzo/commerce-ui'

interface StatCardProps {
  label: string
  value: string | number
  icon?: React.ReactNode
  loading?: boolean
  href?: string
}

export function StatCard({ label, value, icon, loading, href }: StatCardProps) {
  const card = (
    <Container className="p-6 transition-colors hover:border-ui-border-strong">
      <div className="flex items-center justify-between">
        <Text size="small" className="text-ui-fg-muted">{label}</Text>
        {icon}
      </div>
      {loading ? (
        <div className="mt-2 h-9 w-24 animate-pulse rounded bg-ui-bg-component" />
      ) : (
        <Heading level="h2" className="mt-2">{String(value)}</Heading>
      )}
    </Container>
  )
  return href ? <Link href={href} className="block">{card}</Link> : card
}
