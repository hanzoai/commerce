'use client'

// Members management for a customer group. This is the ONE home for membership —
// backed by the group-scoped member endpoints, composed onto the generic CRUD
// hooks (no new data layer):
//   useList(memberKind)   → GET    /v1/customergroup/:id/members
//   add.mutate({userId})  → POST   /v1/customergroup/:id/members
//   remove.mutate(userId) → DELETE /v1/customergroup/:id/members/:userId
// Members + the customer directory load in parallel (no waterfall); userIds are
// resolved to names/emails through an in-memory map of the directory.
import { useMemo, useState } from 'react'
import { Button, Container, Heading, Select, Text, toast } from '@hanzo/commerce-ui'
import { useCreate, useDelete, useList } from '@/lib/api/hooks'
import { customerName, memberKind, type Customer, type Membership } from '@/lib/api/models'
import { errorMessage } from '@/lib/forms/schema'

interface MembersSectionProps {
  groupId: string
  members: Membership[]
  membersLoading: boolean
}

export function MembersSection({ groupId, members, membersLoading }: MembersSectionProps) {
  const kind = memberKind(groupId)
  const { data: directory, isLoading: directoryLoading } = useList<Customer>('c/user', { display: 200 })
  const add = useCreate<Membership>(kind)
  const remove = useDelete(kind)
  const [selected, setSelected] = useState('')

  const customers = directory?.models ?? []
  const byId = useMemo(() => new Map(customers.map((c) => [c.id, c])), [customers])
  const memberIds = useMemo(() => new Set(members.map((m) => m.userId)), [members])
  const candidates = useMemo(() => customers.filter((c) => !memberIds.has(c.id)), [customers, memberIds])

  const onAdd = async () => {
    if (!selected) return
    try {
      await add.mutateAsync({ userId: selected } as Partial<Membership>)
      toast.success('Member added')
      setSelected('')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not add member'))
    }
  }

  const onRemove = async (userId: string) => {
    try {
      await remove.mutateAsync(userId)
      toast.success('Member removed')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not remove member'))
    }
  }

  return (
    <Container className="divide-y p-0">
      <div className="flex items-center justify-between gap-2 px-6 py-4">
        <Heading level="h2">Members</Heading>
        <Text size="small" className="text-ui-fg-muted">
          {members.length} {members.length === 1 ? 'member' : 'members'}
        </Text>
      </div>

      <div className="flex items-center gap-2 px-6 py-4">
        <div className="flex-1">
          <Select value={selected} onValueChange={setSelected} disabled={directoryLoading || candidates.length === 0}>
            <Select.Trigger>
              <Select.Value
                placeholder={
                  directoryLoading
                    ? 'Loading customers…'
                    : candidates.length
                      ? 'Select a customer to add…'
                      : 'Every customer is already a member'
                }
              />
            </Select.Trigger>
            <Select.Content>
              {candidates.map((c) => (
                <Select.Item key={c.id} value={c.id}>
                  {customerName(c)}
                  {c.email && c.email !== customerName(c) ? ` · ${c.email}` : ''}
                </Select.Item>
              ))}
            </Select.Content>
          </Select>
        </div>
        <Button size="small" variant="secondary" onClick={onAdd} disabled={!selected} isLoading={add.isPending}>
          Add
        </Button>
      </div>

      {membersLoading ? (
        <div className="px-6 py-6">
          <Text size="small" className="text-ui-fg-muted">Loading members…</Text>
        </div>
      ) : members.length === 0 ? (
        <div className="px-6 py-6">
          <Text size="small" className="text-ui-fg-muted">No members yet. Add a customer above.</Text>
        </div>
      ) : (
        members.map((m) => {
          const c = byId.get(m.userId)
          return (
            <div key={m.id} className="flex items-center justify-between gap-2 px-6 py-3">
              <div className="min-w-0">
                <Text size="small" weight="plus" leading="compact" className="truncate text-ui-fg-base">
                  {c ? customerName(c) : m.userId}
                </Text>
                {c?.email ? (
                  <Text size="xsmall" leading="compact" className="truncate text-ui-fg-muted">{c.email}</Text>
                ) : null}
              </div>
              <Button size="small" variant="transparent" onClick={() => onRemove(m.userId)} disabled={remove.isPending}>
                Remove
              </Button>
            </div>
          )
        })
      )}
    </Container>
  )
}

export default MembersSection
