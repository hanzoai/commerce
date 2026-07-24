'use client'

import { useState } from 'react'
import dynamic from 'next/dynamic'
import { useParams, useRouter } from 'next/navigation'
import { Button, StatusBadge, Text, toast, usePrompt } from '@hanzo/commerce-ui'
import { DetailShell } from '@/components/common/detail-view/detail-shell'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import { ToasterMount } from '@/components/common/toaster-mount'
import { useGet, useDelete } from '@/lib/api/hooks'
import { customerName, type Customer } from '@/lib/api/models'
import { errorMessage } from '@/lib/forms/schema'

// Editing is a rarely-used panel — defer its (react-hook-form + zod) chunk.
const CustomerEditForm = dynamic(
  () => import('@/components/customers/customer-edit-form').then((m) => m.CustomerEditForm),
  {
    ssr: false,
    loading: () => (
      <div className="px-6 py-6">
        <Text size="small" className="text-ui-fg-muted">Loading…</Text>
      </div>
    ),
  },
)

export function CustomerDetail() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()
  const prompt = usePrompt()
  const [editing, setEditing] = useState(false)

  const { data: customer, isLoading, isError } = useGet<Customer>('c/user', id)
  const { mutateAsync: remove } = useDelete('c/user')

  const handleDelete = async () => {
    if (!customer) return
    const label = customer.email || customerName(customer)
    const confirmed = await prompt({
      title: 'Delete customer',
      description: `Delete ${label}? This permanently removes the customer and cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
      verificationText: customer.email || undefined,
      verificationInstruction: 'Type the email to confirm:',
    })
    if (!confirmed) return
    try {
      await remove(customer.id)
      toast.success('Customer deleted')
      router.push('/customers')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not delete customer'))
    }
  }

  const fullName = customer ? [customer.firstName, customer.lastName].filter(Boolean).join(' ') : ''
  const joined = customer?.createdAt ? new Date(customer.createdAt).toLocaleDateString() : '-'

  const actions = customer ? (
    <>
      <Button size="small" variant="secondary" onClick={() => setEditing((v) => !v)}>
        {editing ? 'Close' : 'Edit'}
      </Button>
      <Button size="small" variant="danger" onClick={handleDelete}>
        Delete
      </Button>
    </>
  ) : null

  return (
    <>
      <ToasterMount />
      <DetailShell
        title={customer ? customer.email || customerName(customer) : 'Customer'}
        subtitle="Customer"
        backHref="/customers"
        backLabel="Customers"
        actions={actions}
        isLoading={isLoading}
        notFound={!isLoading && (isError || !customer)}
        notFoundLabel="This customer could not be found."
      >
        {customer &&
          (editing ? (
            <Section title="Edit customer">
              <CustomerEditForm customer={customer} onDone={() => setEditing(false)} />
            </Section>
          ) : (
            <Section
              title="General"
              action={
                customer.enabled != null ? (
                  <StatusBadge color={customer.enabled ? 'green' : 'grey'}>
                    {customer.enabled ? 'Active' : 'Inactive'}
                  </StatusBadge>
                ) : undefined
              }
            >
              <SectionRow title="Name" value={fullName || '-'} />
              <SectionRow title="Email" value={customer.email || '-'} />
              <SectionRow title="Company" value={customer.company || '-'} />
              <SectionRow title="Phone" value={customer.phone || '-'} />
              {customer.username ? <SectionRow title="Username" value={customer.username} /> : null}
              <SectionRow title="Joined" value={joined} />
            </Section>
          ))}
      </DetailShell>
    </>
  )
}

export default CustomerDetail
