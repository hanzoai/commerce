// The ONE onboarding step registry — progress indicator, wizard nav, and the
// page all derive from this list so a step is added/renamed in exactly one place.
export interface StepMeta {
  key: 'store' | 'product' | 'payments' | 'subscribe' | 'launch'
  title: string
  /** Short label under the progress indicator. */
  label: string
}

export const STEPS: StepMeta[] = [
  { key: 'store', title: 'Create your store', label: 'Store' },
  { key: 'product', title: 'Add your first product', label: 'Product' },
  { key: 'payments', title: 'Connect a payment provider', label: 'Payments' },
  { key: 'subscribe', title: 'Start your trial', label: 'Plan' },
  { key: 'launch', title: 'Launch', label: 'Launch' },
]

export const STEP_COUNT = STEPS.length
