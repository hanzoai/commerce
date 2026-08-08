package billing

import (
	"context"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/events"
	"github.com/hanzoai/commerce/log"
)

// The money alarm: a charge SETTLED at the processor and the ledger did not move.
//
// Every place that can happen already detected it exactly — six of them, each
// ending at `log.Error("RECONCILE: ...")`. Precise detection that reached
// nobody: a $99 subscription charge settled on 2026-08-06 and sat uncredited for
// about twenty-one hours, and the line naming it had been written the instant it
// happened. The gap was never the detection. It was that a log line is not a
// channel to a person.
//
// So the alarm goes out on the same spine the rest of billing already uses (the
// analytics collector, one best-effort event) where the fleet read side can see
// it and page on it. The log line STAYS: the event is fire-and-forget by design
// and must never be the only record of money that moved.
//
// It reports a defect, not a lifecycle step, which is why it is the one billing
// event carrying `terminal` — the two classes need different humans. A retryable
// one clears when the customer or the provider tries again; a terminal one needs
// somebody to grant the balance and refund the charge, and telling that customer
// to "try again" charges their card twice.
func uncredited(c *zip.Ctx, orgName, subject, settlement, reason string, amountCents int64, terminal bool) {
	log.Error("RECONCILE: a settled payment did not move the ledger — org=%s subject=%s settlement=%s amount=%d terminal=%v: %s",
		orgName, subject, settlement, amountCents, terminal, reason, c)
	fireEvent(c, func(ctx context.Context, ev *events.Client) {
		_ = ev.EmitPaymentUncredited(ctx, orgName, subject, settlement, reason, amountCents, terminal)
	})
}
