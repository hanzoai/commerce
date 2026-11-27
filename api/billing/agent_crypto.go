package billing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/models/pendingintent"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	types "github.com/hanzoai/commerce/types"
)

// Subscriptions paid in stablecoins. Stripe takes no stablecoin charge without the payer
// present, so each period is paid on-session: when a renewal falls due an invoice is raised
// as for a card, a deposit is opened for exactly its USD amount, and the subscription goes
// past due with renewal.action_required carrying the addresses to pay. The period starts only
// when Stripe reports the deposit succeeded, once; an unpaid deposit leaves the subscription
// past due under the same dunning a card follows.

// stablecoinMethod is the payment method type of a subscription that pays by deposit.
const stablecoinMethod = "stablecoin"

// depositMethod is the subject's payment method for paying by deposit, made once.
func depositMethod(db *datastore.Datastore, subject string) (*paymentmethod.PaymentMethod, error) {
	pm := paymentmethod.New(db)
	if found, err := pm.Query().Filter("CustomerId=", subject).Filter("Type=", stablecoinMethod).Get(); err != nil {
		return nil, err
	} else if found {
		return pm, nil
	}
	pm = paymentmethod.New(db)
	pm.CustomerId, pm.UserId = subject, subject
	pm.Type, pm.ProviderRef, pm.ProviderType = stablecoinMethod, stablecoinMethod, string(processor.Stripe)
	pm.Name = "Stablecoin deposit"
	pm.Metadata = map[string]interface{}{"customerId": subject}
	return pm, pm.Create()
}

// PayAction is the machine-readable step that pays a deposit: send the amount to an address.
type PayAction struct {
	Type        string                     `json:"type"` // "send_deposit"
	IntentID    string                     `json:"intentId"`
	AmountCents int64                      `json:"amountCents"`
	Currency    string                     `json:"currency"`
	ExpiresAt   string                     `json:"expiresAt"`
	Addresses   []processor.DepositAddress `json:"addresses"`
}

func payAction(dep *processor.StablecoinDeposit, cents int64) PayAction {
	return PayAction{Type: "send_deposit", IntentID: dep.ID, AmountCents: cents, Currency: "usd",
		ExpiresAt: time.Now().Add(delayedAbandonAfter).UTC().Format(time.RFC3339), Addresses: dep.Addresses}
}

// collectByDeposit is the renewal charge of a subscription that pays by stablecoin. A deposit
// for this invoice that has succeeded pays it; one still open is waited for; with none, one
// is opened and announced. Either way an unpaid renewal answers errWaitingOnBuyer, which
// leaves the invoice open and the subscription past due.
func collectByDeposit(ctx context.Context, org *organization.Organization, sub *subscription.Subscription, pm *paymentmethod.PaymentMethod, inv *billinginvoice.BillingInvoice, amountCents int64) (string, error) {
	reg := processorsForOrg(org)
	if open := finishedInvoiceIntent(inv); open != nil {
		res, err, settled := payInvoiceIntent(ctx, reg, pm, open, amountCents, currency.USD, fmt.Sprintf("Subscription renewal invoice %s", inv.NumberStr), false)
		if !settled {
			if err == nil {
				err = errWaitingOnBuyer
			}
			return "", err
		}
		if perr := paymentorg.Put(res.ProcessorRef, org.Name, inv.UserId, inv.Id(), 0, org.TestMode()); perr != nil {
			log.Warn("renewal: payment %s: the index to invoice %s was not written: %v", res.ProcessorRef, inv.Id(), perr)
		}
		return res.ProcessorRef, nil
	}
	dep, network, f := stripeDeposit(ctx, org, inv.UserId, amountCents, "", fmt.Sprintf("renewal:%s:%d", inv.Id(), inv.AttemptCount),
		fmt.Sprintf("Subscription renewal invoice %s", inv.NumberStr))
	if f != nil {
		return "", fmt.Errorf("no deposit could be opened for the renewal: %s", f.Message)
	}
	if err := pendingintent.PutRef(dep.ID, org.Name, strings.ToLower(inv.UserId), pendingintent.Invoice, inv.Id(), amountCents, "usd", org.TestMode()); err != nil {
		cancelDeposit(ctx, org, dep.ID)
		return "", fmt.Errorf("the renewal's deposit could not be recorded: %w", err)
	}
	if err := pendingintent.Bind(dep.ID, pendingintent.Terms{Delayed: true}); err != nil {
		log.Warn("renewal deposit %s: the terms were not bound: %v", dep.ID, err)
	}
	announce(ctx, sub, "deposit_required", "the renewal is paid by stablecoin: send the deposit to finish it", inv.Id(),
		map[string]interface{}{"type": "send_deposit", "network": network, "pay": payAction(dep, amountCents)})
	return "", fmt.Errorf("%w: renewal of %s waits for a stablecoin deposit", errWaitingOnBuyer, sub.Id())
}

// announce emits renewal.action_required: the invoice, a code to branch on, and the action
// that lets the agent or the buyer satisfy it.
func announce(ctx context.Context, sub *subscription.Subscription, code, why, invoiceID string, action map[string]interface{}) {
	if _, err := engine.EmitBillingEvent(webhookGuardDB(ctx), "renewal.action_required", "subscription", sub.Id(), sub.UserId,
		map[string]interface{}{"invoiceId": invoiceID, "subscriptionId": sub.Id(), "code": code, "reason": why, "action": action}, nil); err != nil {
		log.Error("renewal %s: the action required was not announced: %v", invoiceID, err)
	}
}

// completePlanByDeposit starts a plan's first period by stablecoin: a deposit for the plan's
// price is opened and recorded against the checkout, and the subscription opens when the
// deposit is reported succeeded.
func completePlanByDeposit(ctx context.Context, org *organization.Organization, subject string, s *checkoutsession.Session, key, network string,
	release func(int, string, string) CompleteOut) CompleteOut {
	if cerr := atCap(subject); cerr != nil {
		return release(429, "too_many_payments", cerr.Error())
	}
	dep, net, f := stripeDeposit(ctx, org, subject, s.TotalCents, network, "checkout:"+s.Id()+":"+key,
		fmt.Sprintf("First period of %s for %s", s.Plan, subject))
	if f != nil {
		return release(f.Status, "stablecoin_unavailable", f.Message)
	}
	if err := pendingintent.PutRef(dep.ID, org.Name, strings.ToLower(subject), pendingintent.Plan, s.Id(), s.TotalCents, "usd", org.TestMode()); err != nil {
		cancelDeposit(ctx, org, dep.ID)
		return release(503, "unavailable", "billing is temporarily unavailable; please retry")
	}
	if err := pendingintent.Bind(dep.ID, pendingintent.Terms{Delayed: true}); err != nil {
		log.Warn("plan deposit %s: the terms were not bound: %v", dep.ID, err)
	}
	got, _, _ := checkoutsession.Change(s.Id(), func(x *checkoutsession.Session) bool { x.Intent = dep.ID; return true })
	if got != nil {
		s = got
	}
	v := checkoutView(s)
	v.Next = &NextAction{Type: "send_deposit", IntentID: dep.ID, Addresses: dep.Addresses,
		Message: fmt.Sprintf("send USD stablecoin on %s or another listed network; the subscription starts when the deposit confirms", net)}
	return CompleteOut{Status: 202, View: v}
}

// deposits says the intent is a stablecoin deposit that finishes a plan or an invoice rather
// than crediting a wallet.
func deposits(p *pendingintent.Intent) bool {
	return p.Purpose == pendingintent.Plan || (p.Purpose == pendingintent.Invoice && p.Delayed)
}

// finishDeposit applies a succeeded deposit to what it pays, once: a first period opens its
// subscription, a renewal pays its invoice and starts its period. Applying it again changes
// nothing. charge is the payment's reference.
func finishDeposit(ctx context.Context, p *pendingintent.Intent, charge string) error {
	org, err := intentOrg(ctx, p.Org, p.Test)
	if err != nil {
		return err
	}
	if p.Done {
		return nil
	}
	switch p.Purpose {
	case pendingintent.Invoice:
		return finishInvoiceDeposit(ctx, org, p)
	case pendingintent.Plan:
		return finishPlanDeposit(ctx, org, p, charge)
	}
	return nil
}

func finishInvoiceDeposit(ctx context.Context, org *organization.Organization, p *pendingintent.Intent) error {
	inv, db, f := loadInvoice(ctx, org, p.Ref)
	if f != nil {
		return fmt.Errorf("the invoice a deposit pays could not be read: %s", f.Message)
	}
	if inv.Status != billinginvoice.Paid {
		res, err := engine.CollectInvoice(ctx, db, inv, prepaidFor(ctx, org), chargeProvider(org, false))
		if err != nil {
			return err
		}
		if !res.Success {
			return fmt.Errorf("the deposit for invoice %s was not applied: %s", inv.Id(), res.Error)
		}
		if err := inv.Update(); err != nil {
			return err
		}
	}
	_, _ = pendingintent.Settle(p.Id())
	// The period starts now that its invoice is paid.
	renewDue(ctx, org, inv.UserId)
	return nil
}

func finishPlanDeposit(ctx context.Context, org *organization.Organization, p *pendingintent.Intent, charge string) error {
	claim, replayed, err := idemBegin(webhookGuardDB(ctx), "billing-intent", p.Id())
	if err != nil {
		return err
	}
	if replayed {
		return nil // another delivery is applying it, or it has been applied
	}
	fail := func(err error) error {
		_ = claim.Delete()
		return err
	}
	s, found, err := checkoutsession.Get(p.Ref)
	if err != nil || !found {
		return fail(fmt.Errorf("the checkout a deposit pays could not be read: %v", err))
	}
	db := datastore.New(org.Namespaced(ctx))
	var pm *paymentmethod.PaymentMethod
	source := "agent/stablecoin"
	// If the settled PaymentIntent attached a reusable Stripe PaymentMethod (such as
	// an ACH us_bank_account with setup_future_usage=off_session), record and attach
	// it so renewals auto-debit that bank account instead of waiting on a deposit.
	if sp, serr := processorsForOrg(org).Existing(ctx, processor.Stripe); serr == nil {
		if tx, terr := sp.GetTransaction(ctx, p.Id()); terr == nil && tx != nil {
			pmRef, _ := tx.Metadata["payment_method"].(string)
			if strings.HasPrefix(pmRef, "pm_") && tx.CustomerID != "" {
				var card processor.Card
				if r, ok := sp.(interface {
					ReadPaymentMethod(context.Context, string) (processor.Card, error)
				}); ok {
					card, _ = r.ReadPaymentMethod(ctx, pmRef)
				}
				if match := duplicateOf(db, s.Subject, processor.Stripe, card); match != nil {
					pm = match
				} else {
					saved := paymentmethod.New(db)
					saved.CustomerId, saved.UserId = s.Subject, s.Subject
					if card.ExpMonth == 0 && card.ExpYear == 0 && card.Brand != "cashapp" {
						saved.Type = "bank_account"
					} else {
						saved.Type = "card"
					}
					saved.ProviderRef, saved.ProviderType = pmRef, string(processor.Stripe)
					saved.Metadata = map[string]interface{}{"customerId": tx.CustomerID, "cardId": pmRef, "verified": true}
					stampCard(saved, card)
					if saved.Create() == nil {
						pm = saved
					}
				}
				if pm != nil {
					source = "subscribe/ach"
				}
			}
		}
	}
	if pm == nil {
		pm, err = depositMethod(db, s.Subject)
		if err != nil {
			return fail(err)
		}
	}
	pl, err := resolveSubscriptionPlan(db, s.Plan)
	if err != nil {
		return fail(err)
	}
	interval := types.Monthly
	if s.Interval == string(types.Yearly) || s.Interval == "year" {
		interval = types.Yearly
	}
	if pl, err = planAt(pl, s.Level, interval); err != nil {
		return fail(err)
	}
	lines, err := linesAt(db, pl, nil, pl.Interval, currency.USD)
	if err != nil {
		return fail(err)
	}
	qty := s.Quantity
	if qty < 1 {
		qty = 1
	}
	sub, err := createSubscription(db, pl, &createSubscriptionRequest{
		UserId: s.Subject, PlanId: s.Plan, DefaultPaymentMethod: pm.Id(), Quantity: qty,
		Metadata: map[string]interface{}{"source": source, "payment": p.Id()}, Test: org.TestMode(), priced: lines,
	})
	if err != nil {
		return fail(err)
	}
	if code := promo.CouponCode(s.Coupon); code != "" {
		if offer, reason := promo.FindCoupon(ctx, code, time.Now()); reason == "" {
			engine.Grant(sub, offer.Coupon(), offer.Plans, sub.PeriodStart)
			_ = sub.Update()
		}
	}
	if source == "subscribe/ach" {
		sub.ProviderType = string(processor.Stripe)
		_ = sub.Update()
	}
	ref := charge
	if ref == "" {
		ref = p.Id()
	}
	invID := ""
	if inv, ierr := engine.CreatePaidFirstInvoice(db, sub, source, ref); ierr == nil && inv != nil {
		invID = inv.Id()
	}
	if charge != "" {
		if err := paymentorg.Put(charge, org.Name, s.Subject, invID, 0, org.TestMode()); err != nil {
			log.Warn("plan deposit %s: the payment index was not written: %v", p.Id(), err)
		}
	}
	checkoutsession.Change(s.Id(), func(x *checkoutsession.Session) bool {
		x.Status, x.PaymentID, x.SubscriptionID, x.CompletedAt, x.Reason = checkoutsession.Complete, charge, sub.Id(), time.Now(), ""
		return true
	})
	_ = idempotencykey.Complete(claim, sub.Id())
	_, _ = pendingintent.Settle(p.Id())
	return nil
}

// ---- a recurring token's room ----

// tokenWarnWindow is how long before a renewal is due an agent is told its token cannot
// cover it, so a fresh one can be supplied before the attempt.
const tokenWarnWindow = 72 * time.Hour

// TokenRoom is what is left on the shared payment token a subscription renews on, and
// whether it covers the next renewal. A recurring token's maximum is spent by every charge
// made on it, so room runs out before the token expires.
type TokenRoom struct {
	RemainingCents    int64  `json:"remainingCents"`
	MaxCents          int64  `json:"maxCents"`
	RenewalCents      int64  `json:"renewalCents"`
	Covers            bool   `json:"covers"`
	RecurringInterval string `json:"recurringInterval,omitempty"`
	ExpiresAt         string `json:"expiresAt,omitempty"`
}

// renewalCents is what the subscription's next renewal charges.
func renewalCents(sub *subscription.Subscription) int64 { return SubscriptionMRRCents(sub) }

// roomOf reads the token behind a payment method and says what it has left for a renewal of
// cents. Nil for a method that is not a token or a token that cannot be read.
func roomOf(ctx context.Context, org *organization.Organization, pm *paymentmethod.PaymentMethod, cents int64) *TokenRoom {
	if pm == nil || pm.Type != agentToken {
		return nil
	}
	sp, err := processorsForOrg(org).Existing(ctx, processor.Stripe)
	gr, ok := sp.(processor.GrantReader)
	if err != nil || !ok {
		return nil
	}
	g, err := gr.Grant(ctx, pm.ProviderRef)
	if err != nil {
		return nil
	}
	left := max(g.MaxAmount-g.Captured, 0)
	r := &TokenRoom{RemainingCents: left, MaxCents: g.MaxAmount, RenewalCents: cents, RecurringInterval: g.RecurringInterval}
	if g.ExpiresAt > 0 {
		r.ExpiresAt = time.Unix(g.ExpiresAt, 0).UTC().Format(time.RFC3339)
	}
	r.Covers = g.Active && left >= cents && g.RecurringInterval != ""
	return r
}

// subscriptionRoom is the room on the token a subscription renews on, nil when it renews
// on anything else.
func subscriptionRoom(ctx context.Context, org *organization.Organization, subID string) *TokenRoom {
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(subID); err != nil || sub.DefaultPaymentMethod == "" {
		return nil
	}
	pm := paymentmethod.New(db)
	if err := pm.GetById(sub.DefaultPaymentMethod); err != nil {
		return nil
	}
	return roomOf(ctx, org, pm, renewalCents(sub))
}

// warnTokenRenewals tells the agent of each subscription renewing on a token that the token
// will not cover the renewal due within tokenWarnWindow: its room is less than the renewal,
// it expires first, or it is no longer active. Once per subscription per period, ahead of
// the attempt, with the path to supply a fresh token.
func warnTokenRenewals(ctx context.Context, org *organization.Organization) {
	subs, err := ListSubscriptions(ctx, org, "", "")
	if err != nil {
		return
	}
	db := datastore.New(org.Namespaced(ctx))
	now := time.Now()
	for _, sub := range subs {
		if sub.Test != org.TestMode() || sub.Status != subscription.Active || sub.DefaultPaymentMethod == "" || sub.PeriodEnd.Sub(now) > tokenWarnWindow || !sub.PeriodEnd.After(now) {
			continue
		}
		pm := paymentmethod.New(db)
		if err := pm.GetById(sub.DefaultPaymentMethod); err != nil || pm.Type != agentToken {
			continue
		}
		cents := renewalCents(sub)
		room := roomOf(ctx, org, pm, cents)
		if room == nil {
			continue
		}
		expires, _ := time.Parse(time.RFC3339, room.ExpiresAt)
		switch {
		case room.Covers && (expires.IsZero() || expires.After(sub.PeriodEnd)):
			continue
		}
		why, code := "the token does not have the room for the next renewal", "token_over_limit"
		if room.RemainingCents >= cents && !expires.IsZero() && !expires.After(sub.PeriodEnd) {
			why, code = "the token expires before the next renewal", "token_expired"
		}
		if rec, replay, gerr := idempotencykey.Begin(db, "billing-token-warn:"+sub.Id(), fmt.Sprint(sub.PeriodEnd.Unix())); gerr != nil || replay {
			continue
		} else {
			_ = idempotencykey.Complete(rec, "warned")
		}
		announce(ctx, sub, code, why, "", map[string]interface{}{
			"type": "supply_token", "method": "POST", "path": "/v1/billing/subscriptions/" + sub.Id() + "/agent-token",
			"requires": map[string]interface{}{"recurringInterval": "month", "minRemainingCents": cents},
			"room":     room, "dueAt": sub.PeriodEnd.UTC().Format(time.RFC3339),
		})
	}
}

// SupplySubscriptionToken replaces the token a subscription renews on with a fresh one that
// covers the next renewal, before the renewal is attempted.
func SupplySubscriptionToken(ctx context.Context, org *organization.Organization, subject, subID, token string) (*TokenRoom, *AgentError, *PaymentFault) {
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(subID); err != nil || !strings.EqualFold(sub.UserId, subject) {
		return nil, nil, fault(404, "subscription not found", err)
	}
	cents := renewalCents(sub)
	g, aerr := readGrant(ctx, org, token, cents, true)
	if aerr != nil {
		return nil, aerr, nil
	}
	pm, err := tokenMethod(db, subject, token, g)
	if err != nil {
		return nil, nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	sub.DefaultPaymentMethod = pm.Id()
	if err := sub.Update(); err != nil {
		_ = pm.Delete()
		return nil, nil, fault(503, "billing is temporarily unavailable; please retry", err)
	}
	return roomOf(ctx, org, pm, cents), nil, nil
}
