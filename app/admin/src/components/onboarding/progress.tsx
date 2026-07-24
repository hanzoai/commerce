'use client'

import { Text, clx } from '@hanzo/commerce-ui'
import { STEPS } from './steps'

// Top progress indicator — a numbered rail derived from the shared STEPS list.
// `current` is the active index; earlier steps read as done, the rest as pending.
// Clicking a completed/earlier step jumps back to it.
export function WizardProgress({
  current,
  onJump,
}: {
  current: number
  onJump?: (index: number) => void
}) {
  return (
    <ol className="flex w-full items-center">
      {STEPS.map((step, index) => {
        const done = index < current
        const active = index === current
        const reachable = index <= current && !!onJump
        return (
          <li key={step.key} className={clx('flex items-center', index < STEPS.length - 1 && 'flex-1')}>
            <button
              type="button"
              disabled={!reachable}
              onClick={reachable ? () => onJump!(index) : undefined}
              className={clx('flex items-center gap-2', reachable && 'cursor-pointer')}
              aria-current={active ? 'step' : undefined}
            >
              <span
                className={clx(
                  'flex h-7 w-7 shrink-0 items-center justify-center rounded-full border text-xs transition-colors',
                  done && 'border-transparent bg-ui-tag-green-bg text-ui-tag-green-text',
                  active && 'border-ui-fg-base bg-ui-bg-base text-ui-fg-base',
                  !done && !active && 'border-ui-border-base bg-ui-bg-component text-ui-fg-muted',
                )}
              >
                {done ? '✓' : index + 1}
              </span>
              <Text
                size="small"
                weight={active ? 'plus' : 'regular'}
                className={clx('hidden sm:block', active ? 'text-ui-fg-base' : 'text-ui-fg-muted')}
              >
                {step.label}
              </Text>
            </button>
            {index < STEPS.length - 1 && (
              <span
                className={clx(
                  'mx-2 h-px flex-1',
                  index < current ? 'bg-ui-tag-green-bg' : 'bg-ui-border-base',
                )}
              />
            )}
          </li>
        )
      })}
    </ol>
  )
}
