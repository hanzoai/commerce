'use client'

// Reusable scaffold for a resource create/edit page: PageHeader + a centered
// form body + a sticky footer with Cancel / Save and an optional confirm-Delete.
// Both the inventory create and edit pages render through this one layout so the
// chrome, spacing, and save/cancel/delete wiring live in exactly one place.

import type { FormEventHandler, ReactNode } from 'react'
import { useRouter } from 'next/navigation'
import { Button, Container } from '@hanzo/commerce-ui'
import { PageHeader } from './page-header'
import { DeleteButton } from './delete-button'

interface ResourceFormLayoutProps {
  title: string
  description?: string
  backHref: string
  onSubmit: FormEventHandler<HTMLFormElement>
  submitLabel: string
  submitting?: boolean
  onDelete?: () => void | Promise<void>
  deleting?: boolean
  deleteLabel?: string
  deleteTitle?: string
  deleteDescription?: string
  children: ReactNode
}

export function ResourceFormLayout({
  title,
  description,
  backHref,
  onSubmit,
  submitLabel,
  submitting,
  onDelete,
  deleting,
  deleteLabel,
  deleteTitle,
  deleteDescription,
  children,
}: ResourceFormLayoutProps) {
  const router = useRouter()
  const goBack = () => router.push(backHref)

  return (
    <form onSubmit={onSubmit} className="flex min-h-full flex-col">
      <PageHeader
        title={title}
        description={description}
        actions={
          <Button type="button" size="small" variant="secondary" onClick={goBack}>
            Back
          </Button>
        }
      />

      <div className="flex-1 p-8">
        <Container className="mx-auto flex w-full max-w-2xl flex-col gap-y-6 p-6">{children}</Container>
      </div>

      <div className="sticky bottom-0 flex items-center justify-between gap-2 border-t border-ui-border-base bg-ui-bg-base px-8 py-4">
        <div>
          {onDelete && (
            <DeleteButton
              onDelete={onDelete}
              loading={deleting}
              label={deleteLabel}
              title={deleteTitle}
              description={deleteDescription}
            />
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button type="button" size="small" variant="secondary" onClick={goBack} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" size="small" isLoading={submitting}>
            {submitLabel}
          </Button>
        </div>
      </div>
    </form>
  )
}
