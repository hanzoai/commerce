'use client'

import { useRouter } from 'next/navigation'
import { Container, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { groupSchema, groupFields, groupDefaults } from '@/components/customer-groups/group-form'
import { useCreate } from '@/lib/api/hooks'
import type { CustomerGroup } from '@/lib/api/models'
import { errorMessage } from '@/lib/forms/schema'

export default function CreateCustomerGroupPage() {
  const router = useRouter()
  const { mutateAsync, isPending } = useCreate<CustomerGroup>('customergroup')

  return (
    <div>
      <ToasterMount />
      <PageHeader title="Create customer group" description="Group customers together" />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          <ResourceForm
            schema={groupSchema}
            defaultValues={groupDefaults()}
            fields={groupFields}
            submitLabel="Create"
            isPending={isPending}
            single
            onCancel={() => router.push('/customer-groups')}
            onSubmit={async (values) => {
              try {
                await mutateAsync({ name: values.name })
                toast.success(`Group "${values.name}" created`)
                router.push('/customer-groups')
              } catch (e) {
                toast.error(errorMessage(e, 'Could not create group'))
              }
            }}
          />
        </Container>
      </div>
    </div>
  )
}
