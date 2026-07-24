'use client'

// Inline edit form for a customer — the same schema/fields as create, bound to an
// UPDATE (PATCH /v1/c/user/:id). Loaded on demand (dynamic import) from the detail
// page so viewing a customer never ships the form chunk.
import { toast } from '@hanzo/commerce-ui'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { customerSchema, customerFields, customerDefaults } from './customer-form'
import { useUpdate } from '@/lib/api/hooks'
import type { Customer } from '@/lib/api/models'
import { cleanEmpty, errorMessage } from '@/lib/forms/schema'

export function CustomerEditForm({ customer, onDone }: { customer: Customer; onDone: () => void }) {
  const { mutateAsync, isPending } = useUpdate<Customer>('c/user')

  return (
    <div className="px-6 py-5">
      <ResourceForm
        schema={customerSchema}
        defaultValues={customerDefaults(customer)}
        fields={customerFields}
        submitLabel="Save"
        isPending={isPending}
        onCancel={onDone}
        onSubmit={async (values) => {
          try {
            await mutateAsync({ id: customer.id, data: cleanEmpty(values) as Partial<Customer> })
            toast.success('Customer updated')
            onDone()
          } catch (e) {
            toast.error(errorMessage(e, 'Could not update customer'))
          }
        }}
      />
    </div>
  )
}

export default CustomerEditForm
