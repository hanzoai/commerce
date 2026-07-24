'use client'

import { useState } from 'react'
import dynamic from 'next/dynamic'
import { useParams, useRouter } from 'next/navigation'
import { Button, Text, toast, usePrompt } from '@hanzo/commerce-ui'
import { DetailShell } from '@/components/common/detail-view/detail-shell'
import { Section } from '@/components/common/detail-view/section'
import { SectionRow } from '@/components/common/section/section-row'
import { ToasterMount } from '@/components/common/toaster-mount'
import { MembersSection } from '@/components/customer-groups/members-section'
import { useGet, useList, useDelete } from '@/lib/api/hooks'
import { memberKind, type CustomerGroup, type Membership } from '@/lib/api/models'
import { errorMessage } from '@/lib/forms/schema'

// Editing is a rarely-used panel — defer its (react-hook-form + zod) chunk.
const GroupEditForm = dynamic(
  () => import('@/components/customer-groups/group-edit-form').then((m) => m.GroupEditForm),
  {
    ssr: false,
    loading: () => (
      <div className="px-6 py-6">
        <Text size="small" className="text-ui-fg-muted">Loading…</Text>
      </div>
    ),
  },
)

export function CustomerGroupDetail() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()
  const prompt = usePrompt()
  const [editing, setEditing] = useState(false)

  const { data: group, isLoading, isError } = useGet<CustomerGroup>('customergroup', id)
  const { data: memberList, isLoading: membersLoading } = useList<Membership>(memberKind(id))
  const { mutateAsync: remove } = useDelete('customergroup')

  const members = memberList?.models ?? []

  const handleDelete = async () => {
    if (!group) return
    const confirmed = await prompt({
      title: 'Delete customer group',
      description: `Delete "${group.name}"? This removes the group and its memberships and cannot be undone.`,
      confirmText: 'Delete',
      cancelText: 'Cancel',
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await remove(group.id)
      toast.success('Group deleted')
      router.push('/customer-groups')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not delete group'))
    }
  }

  const created = group?.createdAt ? new Date(group.createdAt).toLocaleDateString() : '-'

  const actions = group ? (
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
        title={group ? group.name : 'Customer group'}
        subtitle="Customer group"
        backHref="/customer-groups"
        backLabel="Customer groups"
        actions={actions}
        isLoading={isLoading}
        notFound={!isLoading && (isError || !group)}
        notFoundLabel="This customer group could not be found."
      >
        {group && (
          <>
            {editing ? (
              <Section title="Edit group">
                <GroupEditForm group={group} onDone={() => setEditing(false)} />
              </Section>
            ) : (
              <Section title="General">
                <SectionRow title="Name" value={group.name} />
                <SectionRow title="Members" value={String(members.length)} />
                <SectionRow title="Created" value={created} />
              </Section>
            )}
            <MembersSection groupId={group.id} members={members} membersLoading={membersLoading} />
          </>
        )}
      </DetailShell>
    </>
  )
}

export default CustomerGroupDetail
