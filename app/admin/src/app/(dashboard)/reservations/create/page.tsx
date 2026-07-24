'use client'

import { ResourceCreate } from '@/components/resource/resource-create'
import { reservationDescriptor } from '@/lib/reservation'

export default function CreateReservationPage() {
  return <ResourceCreate descriptor={reservationDescriptor} description="Reserve stock of an item at a location." />
}
