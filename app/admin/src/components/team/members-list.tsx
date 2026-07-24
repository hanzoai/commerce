'use client'

// The team roster. Members come from `useTeamMembers()` — today a projection of the
// signed-in user, because no org-members API is reachable from the browser (see
// lib/api/team.ts). The div-list markup mirrors the customer-group MembersSection so
// membership UI reads the same everywhere.
import { Badge, Button, Container, Heading, Text } from '@hanzo/commerce-ui'
import { useTeamMembers, type TeamMember, type TeamRole } from '@/lib/api/team'

const ROLE_COLOR: Record<TeamRole, 'purple' | 'blue' | 'grey'> = {
  owner: 'purple',
  admin: 'blue',
  member: 'grey',
}

const ROLE_LABEL: Record<TeamRole, string> = {
  owner: 'Owner',
  admin: 'Admin',
  member: 'Member',
}

export function MembersList() {
  const { members, isLoading, partial } = useTeamMembers()

  return (
    <Container className="divide-y p-0">
      <div className="flex items-center justify-between gap-2 px-6 py-4">
        <Heading level="h2">Members</Heading>
        <Text size="small" className="text-ui-fg-muted">
          {members.length} {members.length === 1 ? 'member' : 'members'}
        </Text>
      </div>

      {isLoading ? (
        <div className="px-6 py-6">
          <Text size="small" className="text-ui-fg-muted">Loading members…</Text>
        </div>
      ) : members.length === 0 ? (
        <div className="px-6 py-6">
          <Text size="small" className="text-ui-fg-muted">No members yet.</Text>
        </div>
      ) : (
        members.map((m) => <MemberRow key={m.id} member={m} />)
      )}

      {partial && !isLoading && (
        <div className="px-6 py-4">
          <Text size="xsmall" className="text-ui-fg-muted">
            Only your own membership is shown. The full team roster is managed by Hanzo
            IAM and isn&apos;t exposed to the browser yet — see the note above.
          </Text>
        </div>
      )}
    </Container>
  )
}

function MemberRow({ member }: { member: TeamMember }) {
  const initial = (member.name[0] || member.email[0] || '?').toUpperCase()
  return (
    <div className="flex items-center justify-between gap-3 px-6 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-ui-bg-component text-sm font-medium text-ui-fg-base">
          {initial}
        </span>
        <div className="min-w-0">
          <Text size="small" weight="plus" leading="compact" className="truncate text-ui-fg-base">
            {member.name}
            {member.you && <span className="ml-2 text-ui-fg-muted">(You)</span>}
          </Text>
          {member.email ? (
            <Text size="xsmall" leading="compact" className="truncate text-ui-fg-muted">{member.email}</Text>
          ) : null}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-3">
        <Badge size="2xsmall" color={ROLE_COLOR[member.role]}>{ROLE_LABEL[member.role]}</Badge>
        <Button
          size="small"
          variant="transparent"
          disabled={member.you}
          title={member.you ? "You can't remove yourself" : undefined}
        >
          Remove
        </Button>
      </div>
    </div>
  )
}

export default MembersList
