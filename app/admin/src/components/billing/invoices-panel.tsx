'use client'

// Invoices: list, download the PDF (auth-bearer fetch → blob), and — where the
// invoice state allows — pay an open invoice or void a draft/open one.

import { useState } from 'react'
import { Badge, Button, Text, toast } from '@hanzo/commerce-ui'
import { Fieldset } from '@/components/common/field'
import { ConfirmButton } from '@/components/common/confirm-button'
import { formatMoney, formatDate } from '@/lib/format'
import { errorMessage } from '@/lib/forms/schema'
import type { Commerce, Invoice } from '@/lib/commerce-client'

const STATUS_COLOR: Record<string, 'green' | 'orange' | 'red' | 'grey'> = {
  paid: 'green',
  open: 'orange',
  past_due: 'red',
  uncollectible: 'red',
  void: 'grey',
  draft: 'grey',
}

interface InvoiceRow extends Invoice {
  numberStr?: string
  amountDue?: number
  amountPaid?: number
}

function invoiceAmount(inv: InvoiceRow): number | null {
  const value = inv.amountDue ?? inv.total ?? inv.amountPaid
  return typeof value === 'number' ? value : null
}

export function InvoicesPanel({
  client,
  invoices,
  onChanged,
}: {
  client: Commerce
  invoices: InvoiceRow[]
  onChanged: () => void
}) {
  const [busyId, setBusyId] = useState('')

  const download = async (inv: InvoiceRow) => {
    setBusyId(inv.id)
    try {
      await client.downloadInvoicePdf(inv.id, inv.numberStr || inv.number || `invoice-${inv.id}`)
    } catch (e) {
      toast.error(errorMessage(e, 'Could not download the invoice'))
    } finally {
      setBusyId('')
    }
  }

  const pay = async (inv: InvoiceRow) => {
    setBusyId(inv.id)
    try {
      const res = await client.payInvoice(inv.id)
      if (res.collection?.success === false) {
        toast.error('Payment was declined. Try another card.')
      } else {
        toast.success('Invoice paid')
      }
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not pay the invoice'))
    } finally {
      setBusyId('')
    }
  }

  const voidInvoice = async (inv: InvoiceRow) => {
    setBusyId(inv.id)
    try {
      await client.voidInvoice(inv.id)
      toast.success('Invoice voided')
      onChanged()
    } catch (e) {
      toast.error(errorMessage(e, 'Could not void the invoice'))
    } finally {
      setBusyId('')
    }
  }

  return (
    <Fieldset title="Invoices" description="Download, pay, or void your invoices.">
      <div className="p-5">
        {invoices.length === 0 ? (
          <Text size="small" className="py-4 text-ui-fg-muted">No invoices yet.</Text>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[42rem]">
              <thead>
                <tr className="border-b border-ui-border-base text-left">
                  {['Invoice', 'Date', 'Status', 'Amount', ''].map((h, i) => (
                    <th key={h || 'actions'} className={i === 3 ? 'pb-2 text-right' : 'pb-2'}>
                      <Text as="span" size="xsmall" weight="plus" className="text-ui-fg-muted">{h}</Text>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {invoices.map((inv) => {
                  const status = String(inv.status || '').toLowerCase()
                  const amount = invoiceAmount(inv)
                  const busy = busyId === inv.id
                  const payable = ['open', 'past_due', 'uncollectible'].includes(status)
                  const voidable = ['draft', 'open'].includes(status)
                  return (
                    <tr key={inv.id} className="border-b border-ui-border-base last:border-0">
                      <td className="py-3">
                        <Text as="span" size="small">{inv.numberStr || inv.number || inv.id.slice(-8)}</Text>
                      </td>
                      <td className="py-3">
                        <Text as="span" size="small" className="text-ui-fg-muted">{formatDate(inv.createdAt)}</Text>
                      </td>
                      <td className="py-3">
                        <Badge color={STATUS_COLOR[status] || 'grey'}>{status || '—'}</Badge>
                      </td>
                      <td className="py-3 text-right">
                        <Text as="span" size="small">{amount != null ? formatMoney(amount, inv.currency) : '—'}</Text>
                      </td>
                      <td className="py-3">
                        <div className="flex items-center justify-end gap-1">
                          <Button size="small" variant="transparent" isLoading={busy} onClick={() => download(inv)}>
                            PDF
                          </Button>
                          {payable && (
                            <Button size="small" variant="secondary" disabled={busy} onClick={() => pay(inv)}>
                              Pay
                            </Button>
                          )}
                          {voidable && (
                            <ConfirmButton
                              onConfirm={() => voidInvoice(inv)}
                              title="Void this invoice?"
                              description="Voiding cancels the invoice. This cannot be undone."
                              confirmText="Void"
                              disabled={busy}
                            >
                              Void
                            </ConfirmButton>
                          )}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Fieldset>
  )
}
