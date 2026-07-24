'use client'

import { PageHeader } from '@/components/common/page-header'
import { DiscountForm } from '@/components/discounts/discount-form'

export default function CreateDiscountPage() {
  return (
    <div>
      <PageHeader title="New discount" description="Create a promotion code or automatic discount." />
      <DiscountForm mode="create" />
    </div>
  )
}
