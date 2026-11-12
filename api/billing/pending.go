package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/nscontext"
)

// actionOf is the authentication step an error or result asks of the buyer, or nil.
func actionOf(result *processor.PaymentResult, err error) *processor.ActionRequired {
	var ar *processor.ActionRequired
	if errors.As(err, &ar) {
		return ar
	}
	if result != nil && errors.As(result.Error, &ar) {
		return ar
	}
	return nil
}

// registerAction records, BEFORE the answer that sends the buyer to their bank,
// whose payment that step is waiting on and what for. The intent's webhook, or the
// reconciler, then puts the money where it belongs even when the buyer finishes
// the step in a tab that never comes back. An intent for a SetupIntent ("seti_…")
// moves no money and is not recorded.
func registerAction(result *processor.PaymentResult, err error, org *organization.Organization, subject, purpose string, cents int64, cur currency.Type) {
	ar := actionOf(result, err)
	if ar == nil || !strings.HasPrefix(ar.ID, "pi_") {
		return
	}
	if perr := pendingintent.Put(ar.ID, org.Name, subject, purpose, cents, string(cur), org.TestMode()); perr != nil {
		log.Error("RECONCILE: the 3-D Secure intent %s for %s could not be recorded, so its payment cannot be attributed if the buyer does not come back: %v", ar.ID, subject, perr)
	}
}

// intentGrace is how long a buyer has to finish their bank's step before the
// reconciler looks at the intent.
const intentGrace = 30 * time.Minute

// reconcileIntents settles every intent a buyer was sent to their bank for and
// that no webhook or sale has settled, once its grace has passed: money taken is
// credited (a wallet's) or refunded (a sale that never finished), and an intent
// that never succeeded is closed. It is what keeps money taken from waiting on an
// alarm.
func reconcileIntents(ctx context.Context) {
	open, err := pendingintent.Open(time.Now().Add(-intentGrace))
	if err != nil {
		log.Error("RECONCILE: the pending intents could not be read: %v", err)
		return
	}
	for _, p := range open {
		reconcileIntent(ctx, p)
	}
}

func reconcileIntent(ctx context.Context, p *pendingintent.Intent) {
	org := organization.New(datastore.New(ctx))
	org.Name = p.Org
	if err := org.GetOrCreate("Name=", p.Org); err != nil {
		log.Warn("reconcile intent %s: org %s: %v", p.Id(), p.Org, err)
		return
	}
	if p.Test {
		cp := org.In(organization.Test)
		org = &cp
	}
	sp, err := payment.ProcessorsForOrg(org).Existing(ctx, processor.Stripe)
	if err != nil {
		return
	}
	tx, err := sp.GetTransaction(ctx, p.Id())
	if err != nil {
		log.Warn("reconcile intent %s: %v", p.Id(), err)
		return
	}
	switch tx.Status {
	case "succeeded":
	case "requires_action", "requires_payment_method", "requires_confirmation", "processing":
		if time.Since(p.CreatedAt) > 48*time.Hour {
			_, _ = pendingintent.Settle(p.Id()) // abandoned: the buyer never finished
		}
		return
	default: // canceled
		_, _ = pendingintent.Settle(p.Id())
		return
	}
	if p.Purpose == pendingintent.Sale {
		// Money taken for a plan its sale never finished is returned.
		if _, found, _ := paymentorg.Get(tx.ProcessorRef); found {
			_, _ = pendingintent.Settle(p.Id())
			return
		}
		res, rerr := sp.Refund(ctx, processor.RefundRequest{TransactionID: p.Id(), Reason: "requested_by_customer", IdempotencyKey: "reconcile-refund:" + p.Id()})
		if rerr != nil || res == nil || !res.Success {
			log.Error("RECONCILE: intent %s took %d %s for a plan that was never bought, and refunding it failed: %v", p.Id(), p.Cents, p.Currency, rerr)
			return
		}
		_, _ = pendingintent.Settle(p.Id())
		return
	}
	ev := &processor.WebhookEvent{
		ID: "reconcile:" + p.Id(), Type: "payment.completed", Processor: processor.Stripe,
		Data: map[string]interface{}{
			"id": tx.ProcessorRef, "status": "COMPLETED", "payment_intent": p.Id(),
			"amount": float64(tx.Amount), "currency": string(tx.Currency), "stripe_test": p.Test,
		},
	}
	db := datastore.New(nscontext.WithNamespace(ctx, strings.ToLower(p.Org)))
	if err := applySettlementEvent(ctx, db, org, ev); err != nil {
		log.Error("RECONCILE: intent %s took %d %s and crediting it failed: %v", p.Id(), p.Cents, p.Currency, err)
	}
}
