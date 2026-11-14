package billing

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/types/currency"
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
//
// A buyer may have only maxUnfinished of them open at once: each is a row the
// reconciler reads and a call to Stripe it makes, and a card that always asks
// can be retried as fast as a client can press. Past it the step is refused.
func registerAction(result *processor.PaymentResult, err error, org *organization.Organization, subject, purpose string, cents int64, cur currency.Type) error {
	return registerActionFor(result, err, org, subject, purpose, "", cents, cur)
}

// registerActionFor is registerAction for an intent that also names what it pays.
func registerActionFor(result *processor.PaymentResult, err error, org *organization.Organization, subject, purpose, ref string, cents int64, cur currency.Type) error {
	ar := actionOf(result, err)
	if ar == nil || !strings.HasPrefix(ar.ID, "pi_") {
		return nil
	}
	if cerr := atCap(subject); cerr != nil {
		return cerr
	}
	if perr := pendingintent.PutRef(ar.ID, org.Name, strings.ToLower(subject), purpose, ref, cents, string(cur), org.TestMode()); perr != nil {
		log.Error("RECONCILE: the 3-D Secure intent %s for %s could not be recorded, so its payment cannot be attributed if the buyer does not come back: %v", ar.ID, subject, perr)
		return errors.New("billing is temporarily unavailable; please retry")
	}
	return nil
}

// errTooManyUnfinished is what a buyer at the cap is told.
var errTooManyUnfinished = errors.New("too many payments are waiting on your bank; finish or cancel them first")

// atCap answers errTooManyUnfinished when subject already has maxUnfinished payments
// waiting on their bank. It is asked BEFORE a new card is charged, because a card that
// needs its bank has by then already made an intent at Stripe, and a refusal after that
// leaves an intent no one holds the secret to. registerActionFor asks it again, which
// is the backstop for two requests that both passed.
func atCap(subject string) error {
	if n, err := pendingintent.CountOpen(strings.ToLower(subject)); err == nil && n >= maxUnfinished {
		return errTooManyUnfinished
	}
	return nil
}

const (
	// maxUnfinished is how many 3-D Secure payments one buyer may have waiting.
	maxUnfinished = 5
	// perOrgPerSweep bounds one org's share of a sweep, so a flood in one org
	// cannot starve the rest; perSweep bounds the whole sweep's calls to Stripe.
	perOrgPerSweep = 20
	perSweep       = 400
	// giveUp is how many failed sweeps close an intent that keeps erroring.
	giveUp = 12
	// abandonAfter is how long an intent may wait on a buyer who never finishes.
	abandonAfter = 48 * time.Hour
	// delayedAbandonAfter is how long a local-method payment may wait: past a voucher's
	// three days, with room for a bank's weekend.
	delayedAbandonAfter = 8 * 24 * time.Hour
	delayedLookEvery    = 6 * time.Hour
	// intentGrace is how long a buyer has to finish their bank's step before the
	// reconciler looks at the intent.
	intentGrace = 30 * time.Minute
)

// reconcileIntents settles every intent a buyer was sent to their bank for and
// that no webhook or sale has settled, once its grace has passed: money taken is
// credited (a wallet's) or refunded (a sale that never finished), and an intent
// that never succeeded is closed. It reads ALL the open intents that are due, takes
// them oldest first within each org and round-robin across orgs, and backs a row
// that errors off — so one org's stuck rows, or rows that wait their 48 hours,
// never hold the rest. A row that keeps erroring is closed with an alarm that names
// the money it may hold.
func reconcileIntents(ctx context.Context) {
	open, err := pendingintent.Open(time.Now().Add(-intentGrace))
	if err != nil {
		log.Error("RECONCILE: the pending intents could not be read: %v", err)
		return
	}
	reconcileAll(ctx, open)
}

// reconcileAll is one sweep over the intents that are due.
func reconcileAll(ctx context.Context, open []*pendingintent.Intent) {
	byOrg := map[string][]*pendingintent.Intent{}
	for _, p := range open {
		byOrg[p.Org] = append(byOrg[p.Org], p)
	}
	orgs := make([]string, 0, len(byOrg))
	for o := range byOrg {
		orgs = append(orgs, o)
	}
	sort.Strings(orgs)
	done := 0
	for round := 0; round < perOrgPerSweep && done < perSweep; round++ {
		for _, o := range orgs {
			rows := byOrg[o]
			if round >= len(rows) || done >= perSweep {
				continue
			}
			done++
			p := rows[round]
			if rerr := reconcileIntent(ctx, p); rerr != nil {
				tries, ferr := pendingintent.Failed(p.Id(), rerr.Error())
				switch {
				case ferr != nil:
					log.Error("RECONCILE: intent %s failed (%v) and its back-off could not be recorded: %v", p.Id(), rerr, ferr)
				case tries >= giveUp:
					// Closed so it stops costing a call per sweep, and said loudly: it may
					// have taken money that nothing here has placed.
					_, _ = pendingintent.Settle(p.Id())
					log.Error("RECONCILE: intent %s (%s, %d %s, %s) failed %d sweeps and is closed — it may hold money taken and not placed; last error: %v",
						p.Id(), p.Purpose, p.Cents, p.Currency, p.Subject, tries, rerr)
				}
			}
		}
	}
}

// intentOrg is the org the intent was recorded in, in the mode it was recorded in.
func intentOrg(ctx context.Context, name string, test bool) (*organization.Organization, error) {
	org := organization.New(datastore.New(ctx))
	org.Name = name
	if found, err := org.Query().Filter("Name=", name).Get(); err != nil || !found {
		return nil, fmt.Errorf("org %q: found %v: %v", name, found, err)
	}
	if test {
		cp := org.In(organization.Test)
		return &cp, nil
	}
	return org, nil
}

// reconcileIntent settles one intent, and answers an error when it could not — the
// caller backs it off.
func reconcileIntent(ctx context.Context, p *pendingintent.Intent) error {
	org, err := intentOrg(ctx, p.Org, p.Test)
	if err != nil {
		return err
	}
	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	if err != nil {
		return err
	}
	tx, err := sp.GetTransaction(ctx, p.Id())
	if err != nil {
		return err
	}
	switch tx.Status {
	case "succeeded":
	case "requires_action", "requires_payment_method", "requires_confirmation", "processing":
		deadline := p.CreatedAt.Add(abandonAfter)
		switch {
		case p.PaidCurrency != "" && p.Delayed:
			// A voucher or a bank transfer (boleto, OXXO, SEPA debit, Pix) is paid days after
			// the buyer left, and cancelling it earlier would void a payment they may
			// still make. Its webhook is the way it settles; this is the backstop, so it
			// looks again every few hours until the voucher's own life is over.
			deadline = p.CreatedAt.Add(delayedAbandonAfter)
		case p.PaidCurrency != "" && p.LockExpiresAt > 0:
			// A card on a locked quote: once the lock lapses the amount charged matches no
			// live quote, so the attempt is abandoned there, not days later. A buyer who
			// returns is re-quoted and shown the new amount before they authorize.
			deadline = time.Unix(p.LockExpiresAt, 0)
		}
		if time.Now().Before(deadline) {
			if p.PaidCurrency != "" && p.Delayed {
				return pendingintent.Wait(p.Id(), time.Now().Add(delayedLookEvery))
			}
			// Only waiting on the buyer: not looked at again before its deadline, so a few
			// waiting rows never crowd out the ones that took money.
			return pendingintent.Wait(p.Id(), deadline)
		}
		{
			// The buyer never finished: cancel it at Stripe so it cannot succeed after
			// commerce stopped waiting, and close the row.
			if c, ok := sp.(interface {
				CancelIntent(context.Context, string) error
			}); ok {
				if cerr := c.CancelIntent(ctx, p.Id()); cerr != nil {
					return cerr
				}
			}
			_, _ = pendingintent.Settle(p.Id())
		}
		return nil
	default: // canceled
		_, _ = pendingintent.Settle(p.Id())
		return nil
	}
	if p.Purpose == pendingintent.Sale || p.Purpose == pendingintent.Invoice {
		// Money taken for a plan or an invoice that never finished is returned.
		if _, found, _ := paymentorg.Get(tx.ProcessorRef); found {
			_, _ = pendingintent.Settle(p.Id())
			return nil
		}
		// The intent's one claim: a sale or a top-up that holds it is using it now,
		// and one that is refunded here can never be redeemed again.
		claim, taken, cerr := idemBegin(webhookGuardDB(ctx), "billing-intent", p.Id())
		if cerr != nil {
			return cerr
		}
		if taken {
			return nil
		}
		res, rerr := sp.Refund(ctx, processor.RefundRequest{TransactionID: p.Id(), Reason: "requested_by_customer", IdempotencyKey: "reconcile-refund:" + p.Id()})
		if rerr != nil || res == nil || !res.Success {
			_ = claim.Delete()
			if rerr == nil {
				rerr = errors.New("the refund did not succeed")
			}
			return fmt.Errorf("refunding %d %s taken for a %s that never finished: %w", p.Cents, p.Currency, p.Purpose, rerr)
		}
		_ = idempotencykey.Complete(claim, "refunded")
		_, _ = pendingintent.Settle(p.Id())
		return nil
	}
	if _, found, _ := paymentorg.Get(tx.ProcessorRef); found {
		_, _ = pendingintent.Settle(p.Id()) // credited by the browser or the webhook
		return nil
	}
	// What Stripe took must be what was registered. A row that does not match is a
	// failure like any other — backed off, and closed with an alarm after giveUp
	// sweeps — rather than an alarm every sweep for good.
	if wantCents, wantCur := p.Charged(); int64(tx.Amount) != wantCents || !strings.EqualFold(string(tx.Currency), wantCur) {
		return fmt.Errorf("intent %s took %d %s but was registered for %d %s", p.Id(), tx.Amount, tx.Currency, wantCents, wantCur)
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
		return fmt.Errorf("crediting %d %s: %w", p.Cents, p.Currency, err)
	}
	return nil
}
