'use client'

import { useRouter } from 'next/navigation'
import { Container, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { customerSchema, customerFields, customerDefaults } from '@/components/customers/customer-form'
import { useCreate } from '@/lib/api/hooks'
import type { Customer } from '@/lib/api/models'
import { cleanEmpty, errorMessage } from '@/lib/forms/schema'

export default function CreateCustomerPage() {
  const router = useRouter()
  const { mutateAsync, isPending } = useCreate<Customer>('c/user')

  return (
    <div>
      <ToasterMount />
      <PageHeader title="Create customer" description="Add a customer to your store" />
      <div className="p-8">
        <Container className="mx-auto max-w-2xl p-6">
          <ResourceForm
            schema={customerSchema}
            defaultValues={customerDefaults()}
            fields={customerFields}
            submitLabel="Create"
            isPending={isPending}
            onCancel={() => router.push('/customers')}
            onSubmit={async (values) => {
              try {
                await mutateAsync(cleanEmpty(values) as Partial<Customer>)
                toast.success(`Customer ${values.email} created`)
                router.push('/customers')
              } catch (e) {
                toast.error(errorMessage(e, 'Could not create customer'))
              }
            }}
          />
        </Container>
      </div>
    </div>
  )
}
