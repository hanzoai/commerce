'use client'

// Two ways a teammate joins the org's store:
//
//  1. Invite by email + role — the intended UX, built on the shared ResourceForm
//     engine (email field + role select). This is the DOCUMENTED BACKEND GAP:
//     neither IAM nor commerce exposes an invite-by-email / role-assignment endpoint
//     to the browser, so submitting explains the working alternative rather than
//     silently pretending to send. (Closing the gap = a thin commerce proxy to
//     IAM's org-members API, or an IAM members endpoint reachable from the browser.)
//
//  2. Redeem an invite code — the GENUINELY-WIRED path. A teammate who received a
//     platform-minted code claims it for their org via POST /v1/commerce/invite/redeem,
//     unlocking shared store access. Reachable by an org admin today.
import { useState } from 'react'
import { Button, Container, Heading, Input, Text, toast } from '@hanzo/commerce-ui'
import { ResourceForm } from '@/components/forms/resource-form/resource-form'
import { inviteSchema, inviteFields, inviteDefaults } from './invite-form'
import { useRedeemInvite } from '@/lib/api/team'
import { errorMessage } from '@/lib/forms/schema'

export function InvitePanel() {
  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <InviteByEmail />
      <RedeemInvite />
    </div>
  )
}

function InviteByEmail() {
  return (
    <Container className="p-0">
      <div className="border-b border-ui-border-base px-6 py-4">
        <Heading level="h2">Invite a teammate</Heading>
        <Text size="small" className="mt-1 text-ui-fg-subtle">
          Send an email invite and pick their role.
        </Text>
      </div>
      <div className="px-6 py-5">
        <ResourceForm
          schema={inviteSchema}
          defaultValues={inviteDefaults()}
          fields={inviteFields}
          submitLabel="Send invite"
          single
          onSubmit={(values) => {
            // Documented gap: no invite-by-email backend is reachable from the
            // browser yet. Be honest and point to the code path that works today.
            toast.info(
              `Email invites are coming soon. For now, share an invite code with ${values.email} — they can redeem it on the right to join your store.`,
            )
          }}
        />
        <Text size="xsmall" className="mt-3 text-ui-fg-muted">
          Email invites and role assignment are managed by Hanzo IAM and aren&apos;t
          exposed to the browser yet.
        </Text>
      </div>
    </Container>
  )
}

function RedeemInvite() {
  const [code, setCode] = useState('')
  const redeem = useRedeemInvite()

  const onRedeem = async () => {
    const trimmed = code.trim()
    if (!trimmed) return
    try {
      await redeem.mutateAsync(trimmed)
      toast.success('Invite redeemed — store access unlocked')
      setCode('')
    } catch (e) {
      toast.error(errorMessage(e, 'Could not redeem invite code'))
    }
  }

  return (
    <Container className="p-0">
      <div className="border-b border-ui-border-base px-6 py-4">
        <Heading level="h2">Have an invite code?</Heading>
        <Text size="small" className="mt-1 text-ui-fg-subtle">
          Redeem a code to join a team&apos;s store.
        </Text>
      </div>
      <div className="flex flex-col gap-3 px-6 py-5">
        <Input
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="INVITE-CODE"
          autoComplete="off"
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              onRedeem()
            }
          }}
        />
        <div className="flex justify-end">
          <Button size="small" variant="primary" onClick={onRedeem} disabled={!code.trim()} isLoading={redeem.isPending}>
            Redeem
          </Button>
        </div>
      </div>
    </Container>
  )
}

export default InvitePanel
