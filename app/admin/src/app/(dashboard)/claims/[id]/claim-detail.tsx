'use client'

// Claim detail: the claimed lines (with their per-line reason and, when the
// order is loaded, unit price), the resolution, and the accept / reject
// decision. The claim, its items, and the referenced order all self-fetch in
// PARALLEL (no waterfall). Accept moves money (refund or replacement order) and
// is admin-gated server-side; the UI only offers the decision while the claim is
// pending. On success every claim query is invalidated, so the status and
// settled amount re-read.

import { useParams, useRouter } from 'next/navigation'
import { Badge, Button, Skeleton, Text, toast } from '@hanzo/commerce-ui'
import { PageHeader } from '@/components/common/page-header'
import { Field, Fieldset } from '@/components/common/field'
import { useGet, useResourceAction, useResourceActionData, useOrder } from '@/lib/api/hooks'
import { formatMoney } from '@/lib/format'
import {
  isOpen,
  lineId,
  lineLabel,
  REASON_LABEL,
  RESOLUTION_LABEL,
  STATUS_COLOR,
  type AcceptResult,
  type Claim,
  type ClaimItem,
  type OrderLine,
  type OrderLite,
} from '@/lib/claims/claim'

export function ClaimDetail() {
  const router = useRouter()
  const params = useParams<{ id: string }>()
  const id = params?.id

  const { data: claim, isLoading, isError } = useGet<Claim>('claim', id)
  const { data: items } = useResourceActionData<ClaimItem[]>('claim', id, 'items')
  const { data: order } = useOrder(claim?.orderId) as { data?: OrderLite }

  const accept = useResourceAction<AcceptResult, Record<string, never>>('claim', id, 'accept')
  const reject = useResourceAction<Claim, Record<string, never>>('claim', id, 'reject')

  if (isLoading) return <DetailSkeleton />

  if (isError || !claim) {
    return (
      <div>
        <PageHeader title="Claim not found" description="It may have been deleted." />
        <div className="p-8">
          <Button size="small" variant="secondary" onClick={() => router.push('/claims')}>
            Back to claims
          </Button>
        </div>
      </div>
    )
  }

  const currency = claim.currencyCode || order?.currency || 'usd'
  const linesById = new Map<string, OrderLine>((order?.items ?? []).map((l) => [lineId(l), l]))
  const open = isOpen(claim)

  const onAccept = async () => {
    try {
      const res = await accept.mutateAsync({})
      toast.success(
        claim.resolution === 'refund'
          ? `Accepted — refunded ${formatMoney(res.amountCents, currency)}`
          : 'Accepted — replacement order created',
      )
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not accept the claim')
    }
  }

  const onReject = async () => {
    try {
      await reject.mutateAsync({})
      toast.success('Claim rejected')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not reject the claim')
    }
  }

  return (
    <div>
      <PageHeader
        title="Claim"
        description={`Order ${claim.orderId?.slice(-8) || '—'} · ${RESOLUTION_LABEL[claim.resolution] ?? claim.resolution}`}
        actions={<Badge color={STATUS_COLOR[claim.status] ?? 'grey'}>{claim.status}</Badge>}
      />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        <Fieldset title="Summary" description="What is being claimed and how it settles.">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <ReadOnly label="Resolution" value={RESOLUTION_LABEL[claim.resolution] ?? claim.resolution} />
            <ReadOnly
              label="Amount"
              value={claim.amountCents ? formatMoney(claim.amountCents, currency) : '—'}
            />
            <ReadOnly label="Status" value={claim.status} />
          </div>
          {claim.reason ? (
            <Field label="Reason">
              <Text size="small" className="text-ui-fg-subtle">
                {claim.reason}
              </Text>
            </Field>
          ) : null}
          {claim.refundId ? <ReadOnly label="Refund" value={claim.refundId} /> : null}
          {claim.replacementOrderId ? (
            <ReadOnly label="Replacement order" value={claim.replacementOrderId} />
          ) : null}
        </Fieldset>

        <Fieldset title="Claimed items" description="The order lines this claim covers.">
          {items && items.length ? (
            <div className="overflow-x-auto rounded-lg border border-ui-border-base">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-ui-border-base bg-ui-bg-subtle text-ui-fg-muted">
                  <tr>
                    <th className="px-3 py-2 font-medium">Item</th>
                    <th className="px-3 py-2 font-medium">Reason</th>
                    <th className="px-3 py-2 text-right font-medium">Qty</th>
                    <th className="px-3 py-2 text-right font-medium">Line</th>
                  </tr>
                </thead>
                <tbody>
                  {items.map((it) => {
                    const line = linesById.get(it.itemId)
                    const label = line ? lineLabel(line) : it.itemId
                    const lineTotal = line ? line.price * it.quantity : undefined
                    return (
                      <tr key={it.id} className="border-b border-ui-border-base last:border-0">
                        <td className="px-3 py-2 text-ui-fg-base">{label}</td>
                        <td className="px-3 py-2 text-ui-fg-muted">{REASON_LABEL[it.reason] ?? it.reason}</td>
                        <td className="px-3 py-2 text-right text-ui-fg-base">{it.quantity}</td>
                        <td className="px-3 py-2 text-right text-ui-fg-base">
                          {lineTotal != null ? formatMoney(lineTotal, currency) : '—'}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <Text size="small" className="text-ui-fg-muted">
              No items on this claim.
            </Text>
          )}
        </Fieldset>

        <Fieldset
          title="Decision"
          description={
            open
              ? claim.resolution === 'refund'
                ? 'Accepting refunds the claimed lines against the order.'
                : 'Accepting creates a replacement order for the claimed lines.'
              : 'This claim has already been resolved.'
          }
        >
          <div className="flex items-center justify-end gap-2">
            <Button
              type="button"
              variant="secondary"
              size="small"
              onClick={onReject}
              isLoading={reject.isPending}
              disabled={!open || accept.isPending}
            >
              Reject
            </Button>
            <Button
              type="button"
              size="small"
              onClick={onAccept}
              isLoading={accept.isPending}
              disabled={!open || reject.isPending}
            >
              Accept
            </Button>
          </div>
        </Fieldset>

        <div className="flex justify-start border-t border-ui-border-base pt-4">
          <Button size="small" variant="secondary" onClick={() => router.push('/claims')}>
            Back to claims
          </Button>
        </div>
      </div>
    </div>
  )
}

function ReadOnly({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-y-1.5">
      <Text size="small" weight="plus" className="text-ui-fg-base">
        {label}
      </Text>
      <Text size="small" className="break-all text-ui-fg-subtle">
        {value}
      </Text>
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div>
      <div className="border-b border-ui-border-base px-8 py-6">
        <Skeleton className="h-7 w-56" />
        <Skeleton className="mt-2 h-4 w-32" />
      </div>
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-6 px-4 py-8 sm:px-8">
        {[0, 1, 2].map((i) => (
          <div key={i} className="rounded-lg border border-ui-border-base bg-ui-bg-subtle p-5">
            <Skeleton className="h-4 w-40" />
            <div className="mt-4 flex flex-col gap-y-3">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
