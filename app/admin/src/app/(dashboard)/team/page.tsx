'use client'

// Team / staff management — inside commerce.hanzo.ai (was punted to
// console.hanzo.ai/team). An org admin sees the member roster, invites teammates,
// and picks roles, all in one place.
//
// Backend reality (see lib/api/team.ts): the member roster is projected from the
// IAM session, invite-by-email + role assignment are the documented gap, and the
// genuinely-wired write is the invite-code redeem (POST /v1/commerce/invite/redeem).
import { Container, Text } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { ToasterMount } from '@/components/common/toaster-mount'
import { MembersList } from '@/components/team/members-list'
import { InvitePanel } from '@/components/team/invite-panel'

export default function TeamPage() {
  return (
    <div>
      <ToasterMount />
      <PageHeader title="Team" description="Manage who can access your store" />
      <div className="space-y-6 p-8">
        <Container className="bg-ui-bg-subtle p-6">
          <Text size="small" weight="plus" className="text-ui-fg-base">Managed by Hanzo IAM</Text>
          <Text size="small" className="mt-1 text-ui-fg-subtle">
            Your team is your organization&apos;s Hanzo IAM membership. Everyone in the
            org shares this store&apos;s access. Today you can redeem an invite code to
            join a store; full member listing, email invites, and role changes are
            coming as IAM exposes them to the browser.
          </Text>
        </Container>

        <MembersList />
        <InvitePanel />
      </div>
    </div>
  )
}
