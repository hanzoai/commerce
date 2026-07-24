// Shared prop contract every wizard step honors — the page owns navigation and
// passes these down so a step never reaches into onboarding state directly.
export interface StepProps {
  onNext: () => void
  onBack?: () => void
  onSkip?: () => void
  isFirst: boolean
  isLast: boolean
}
