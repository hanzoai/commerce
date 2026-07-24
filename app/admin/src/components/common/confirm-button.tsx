'use client'

import { Button, usePrompt } from '@hanzo/commerce-ui'
import { useState } from 'react'

interface ConfirmButtonProps {
  onConfirm: () => Promise<void> | void
  title: string
  description: string
  confirmText?: string
  cancelText?: string
  children: React.ReactNode
  variant?: 'primary' | 'secondary' | 'transparent' | 'danger'
  size?: 'small' | 'base' | 'large' | 'xlarge'
  disabled?: boolean
}

/**
 * A button that asks for confirmation (danger prompt) before running an action.
 * The one delete-with-confirm control shared across resources. Manages its own
 * pending state so callers just hand it an async `onConfirm`.
 */
export function ConfirmButton({
  onConfirm,
  title,
  description,
  confirmText = 'Delete',
  cancelText = 'Cancel',
  children,
  variant = 'danger',
  size = 'small',
  disabled,
}: ConfirmButtonProps) {
  const prompt = usePrompt()
  const [busy, setBusy] = useState(false)

  const run = async () => {
    const ok = await prompt({ title, description, variant: 'danger', confirmText, cancelText })
    if (!ok) return
    try {
      setBusy(true)
      await onConfirm()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Button type="button" variant={variant} size={size} onClick={run} isLoading={busy} disabled={disabled}>
      {children}
    </Button>
  )
}
