'use client'

// Reusable "delete with confirm" control. Renders a danger button that opens a
// lightweight confirmation dialog (same overlay pattern the live create-product
// modal uses) before invoking the destructive action. One place for every
// delete-with-confirm flow in the admin.

import { useState } from 'react'
import { Button, Heading, Text } from '@hanzo/commerce-ui'

interface DeleteButtonProps {
  onDelete: () => void | Promise<void>
  loading?: boolean
  label?: string
  title?: string
  description?: string
  confirmLabel?: string
}

export function DeleteButton({
  onDelete,
  loading,
  label = 'Delete',
  title = 'Delete this record?',
  description = 'This action cannot be undone.',
  confirmLabel = 'Delete',
}: DeleteButtonProps) {
  const [open, setOpen] = useState(false)

  const close = () => {
    if (loading) return
    setOpen(false)
  }

  const confirm = async () => {
    await onDelete()
    setOpen(false)
  }

  return (
    <>
      <Button type="button" size="small" variant="danger" onClick={() => setOpen(true)}>
        {label}
      </Button>
      {open && (
        <div
          className="fixed inset-0 z-[70] flex items-center justify-center bg-ui-bg-overlay p-4"
          role="presentation"
          onMouseDown={close}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-confirm-title"
            className="w-full max-w-md rounded-xl border border-ui-border-base bg-ui-bg-subtle p-6 shadow-elevation-modal"
            onMouseDown={(event) => event.stopPropagation()}
          >
            <Heading id="delete-confirm-title" level="h2">
              {title}
            </Heading>
            <Text size="small" className="mt-1 text-ui-fg-subtle">
              {description}
            </Text>
            <div className="mt-6 flex justify-end gap-2">
              <Button type="button" size="small" variant="secondary" onClick={close} disabled={loading}>
                Cancel
              </Button>
              <Button type="button" size="small" variant="danger" onClick={confirm} isLoading={loading}>
                {confirmLabel}
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
