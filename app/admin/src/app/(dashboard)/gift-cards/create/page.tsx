'use client'

import { PageHeader } from '@/components/common/page-header'
import { GiftCardForm } from '@/components/gift-cards/gift-card-form'

export default function CreateGiftCardPage() {
  return (
    <div>
      <PageHeader title="New gift card" description="Issue a prepaid, code-addressable balance." />
      <div className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-8">
        <GiftCardForm mode="create" />
      </div>
    </div>
  )
}
