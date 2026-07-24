'use client'

import { Button, Heading, Text } from '@hanzo/commerce-ui'

// ONE frame + ONE nav for every step — the steps supply only their title, body,
// and which actions to wire. Keeps the wizard DRY and visually consistent.
export function WizardStep({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <div>
      <Heading level="h2">{title}</Heading>
      {description && <Text className="mt-2 text-ui-fg-muted">{description}</Text>}
      <div className="mt-6">{children}</div>
    </div>
  )
}

export interface StepNavProps {
  onBack?: () => void
  onSkip?: () => void
  onNext?: () => void
  nextLabel?: string
  nextDisabled?: boolean
  nextLoading?: boolean
  backLabel?: string
  skipLabel?: string
  hideBack?: boolean
  hideSkip?: boolean
}

export function StepNav({
  onBack,
  onSkip,
  onNext,
  nextLabel = 'Continue',
  nextDisabled,
  nextLoading,
  backLabel = 'Back',
  skipLabel = 'Skip',
  hideBack,
  hideSkip,
}: StepNavProps) {
  return (
    <div className="mt-8 flex items-center justify-between gap-3 border-t border-ui-border-base pt-6">
      <div>
        {!hideBack && onBack && (
          <Button variant="secondary" size="small" onClick={onBack}>
            {backLabel}
          </Button>
        )}
      </div>
      <div className="flex items-center gap-3">
        {!hideSkip && onSkip && (
          <Button variant="transparent" size="small" onClick={onSkip}>
            {skipLabel}
          </Button>
        )}
        {onNext && (
          <Button
            variant="primary"
            size="small"
            onClick={onNext}
            disabled={nextDisabled || nextLoading}
            isLoading={nextLoading}
          >
            {nextLabel}
          </Button>
        )}
      </div>
    </div>
  )
}
