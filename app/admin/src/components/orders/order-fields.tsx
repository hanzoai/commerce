'use client'

import type { Control } from 'react-hook-form'
import { TextField, TextareaField } from '@/components/common/form-fields'
import { SelectField } from '@/components/common/select-field'
import { CURRENCY_OPTIONS, STATUS_OPTIONS } from './order-form'

// The order form field groups, shared by create + edit. Typed against the loose
// form value shape (both forms share the email/status/company/note core) so one
// set of fields serves both instead of copy-pasting per page.

export function OrderDetailsFields({ control, currency }: { control: Control<any>; currency?: boolean }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <TextField
        control={control}
        name="email"
        label="Email"
        type="email"
        placeholder="customer@example.com"
        className="sm:col-span-2"
      />
      {currency && (
        <SelectField control={control} name="currency" label="Currency" options={CURRENCY_OPTIONS} placeholder="Currency" />
      )}
      <SelectField control={control} name="status" label="Status" options={STATUS_OPTIONS} placeholder="Status" />
      <TextField control={control} name="company" label="Company" optional className="sm:col-span-2" />
      <TextareaField control={control} name="giftMessage" label="Note" optional className="sm:col-span-2" />
    </div>
  )
}

export function AddressFields({
  control,
  prefix,
}: {
  control: Control<any>
  prefix: 'shippingAddress' | 'billingAddress'
}) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <TextField control={control} name={`${prefix}.name`} label="Name" optional className="sm:col-span-2" />
      <TextField control={control} name={`${prefix}.line1`} label="Address line 1" optional className="sm:col-span-2" />
      <TextField control={control} name={`${prefix}.line2`} label="Address line 2" optional className="sm:col-span-2" />
      <TextField control={control} name={`${prefix}.city`} label="City" optional />
      <TextField control={control} name={`${prefix}.state`} label="State / Province" optional />
      <TextField control={control} name={`${prefix}.postalCode`} label="Postal code" optional />
      <TextField control={control} name={`${prefix}.country`} label="Country" optional />
    </div>
  )
}
