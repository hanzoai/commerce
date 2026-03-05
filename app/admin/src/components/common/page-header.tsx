import { Heading, Text } from '@hanzo/commerce-ui'

interface PageHeaderProps {
  title: string
  description?: string
  actions?: React.ReactNode
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between border-b border-ui-border-base px-8 py-6">
      <div>
        <Heading level="h1">{title}</Heading>
        {description && <Text size="small" leading="compact" className="mt-1 text-ui-fg-subtle">{description}</Text>}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
