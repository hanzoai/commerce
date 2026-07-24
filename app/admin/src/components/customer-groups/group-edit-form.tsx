'use client'

// Inline edit form for a customer group — same schema/fields as create, bound to an
// UPDATE (PATCH /v1/customergroup/:id). Loaded on demand from the detail page.
import { toast } from '@hanzo/commerce-ui'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { groupSchema, groupFields, groupDefaults } from './group-form'
import { useUpdate } from '@/lib/api/hooks'
import type { CustomerGroup } from '@/lib/api/models'
import { errorMessage } from '@/lib/forms/schema'

export function GroupEditForm({ group, onDone }: { group: CustomerGroup; onDone: () => void }) {
  const { mutateAsync, isPending } = useUpdate<CustomerGroup>('customergroup')

  return (
    <div className="px-6 py-5">
      <ResourceForm
        schema={groupSchema}
        defaultValues={groupDefaults(group)}
        fields={groupFields}
        submitLabel="Save"
        isPending={isPending}
        single
        onCancel={onDone}
        onSubmit={async (values) => {
          try {
            await mutateAsync({ id: group.id, data: { name: values.name } })
            toast.success('Group updated')
            onDone()
          } catch (e) {
            toast.error(errorMessage(e, 'Could not update group'))
          }
        }}
      />
    </div>
  )
}

export default GroupEditForm
