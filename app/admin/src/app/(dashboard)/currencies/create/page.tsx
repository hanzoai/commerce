'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { currencyDescriptor } from '@/lib/currency'

export default function CreateCurrencyPage() {
  return (
    <ResourceCreate
      descriptor={currencyDescriptor}
      title="Enable currency"
      description="Add a currency your store accepts."
      submitLabel="Enable"
    />
  )
}
