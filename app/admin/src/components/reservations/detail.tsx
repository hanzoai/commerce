'use client'

import { ResourceEdit } from '@/components/resource/resource-edit'
import { reservationDescriptor, type Reservation } from '@/lib/reservation'
import { ReservationAdjust } from './adjust-panel'

export function ReservationDetail() {
  return (
    <ResourceEdit
      descriptor={reservationDescriptor}
      description="Edit this reservation or adjust its reserved quantity."
      extra={(record: Reservation) => <ReservationAdjust id={record.id} current={record.quantity ?? 0} />}
    />
  )
}
