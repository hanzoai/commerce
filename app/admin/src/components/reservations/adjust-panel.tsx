'use client'

// The reserved-quantity adjust control for one reservation, posting to
// /v1/reservation/:id/adjust. Shown below the general fields on the reservation
// detail surface via the generic <ResourceEdit> `extra` slot. On success the
// resource-action hook invalidates the reservation queries so the field re-reads.
import { useState } from 'react'
import { Button, Input, Text, toast } from '@hanzo/commerce-ui'
import { Section } from '@/components/common/detail-view/section'
import { useResourceAction } from '@/lib/api/hooks'
import { errorMessage } from '@/lib/forms/schema'
import type { Reservation } from '@/lib/reservation'

export function ReservationAdjust({ id, current }: { id: string; current: number }) {
  const [quantity, setQuantity] = useState(String(current))
  const adjust = useResourceAction<Reservation, { quantity: number }>('reservation', id, 'adjust')

  const submit = async () => {
    const q = Number(quantity)
    if (!Number.isInteger(q) || q < 0) {
      toast.error('Enter a whole, non-negative quantity')
      return
    }
    try {
      await adjust.mutateAsync({ quantity: q })
      toast.success('Reservation adjusted')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not adjust reservation'))
    }
  }

  return (
    <Section title="Adjust reserved quantity">
      <div className="flex items-end gap-3 px-6 py-4">
        <div className="flex flex-col gap-1">
          <Text size="xsmall" className="text-ui-fg-muted">Reserved units</Text>
          <Input
            type="number"
            inputMode="numeric"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            className="w-32"
          />
        </div>
        <Button size="small" variant="secondary" onClick={submit} isLoading={adjust.isPending}>
          Adjust
        </Button>
      </div>
    </Section>
  )
}
