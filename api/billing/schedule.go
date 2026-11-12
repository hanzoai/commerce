package billing

// schedule.go — a card plan that starts when the plan held now runs out.
//
// A plan paid outside checkout (a wire, an invoice raised in the processor's own
// dashboard, a comp) never renews: its next period arrives only when the next
// payment is recorded, and with none it lapses at its end (engine.Over). Nor does a
// plan the balance used to renew: a plan's period is never paid from prepaid money
// (engine.CollectInvoice). Such a subscriber who subscribes by card is not charged
// today for a period already covered. The card is kept on file now, the card plan
// opens SCHEDULED at the held plan's end, and the renewal sweep charges the card
// for its first period at that start, as it charges a trial's first paid period.
// The held plan is set to end there, so the two never overlap and nothing is paid
// twice.
//
// A scheduled row confers nothing before its start: every reader of a plan asks
// for an active or trialing row, and this is neither. Its first charge declining
// leaves it past due, as any renewal's does.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// prepaidSource reports whether a sale's source names prepaid money rather than a
// card. A plan is never bought with it.
func prepaidSource(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "balance", "credits", "credit", "prepaid":
		return true
	}
	return false
}

// prepaidRenewed reports whether a row was opened to renew from prepaid money —
// bought from the balance, or with a wallet that keeps no card — which no
// renewal does any more: its next period is charged to a card on file, and it
// holds none.
func prepaidRenewed(s *subscription.Subscription) bool {
	return s.Type != subscription.External && strings.EqualFold(strings.TrimSpace(s.DefaultPaymentMethod), "credits")
}

// followed reports whether a card sale for the subject holding s starts when s
// ends rather than today: s is active, will not renew by card, and has not ended.
func followed(s *subscription.Subscription) bool {
	return s.Status == subscription.Active && !s.EndCancel &&
		(s.Type == subscription.External || prepaidRenewed(s)) &&
		s.PeriodEnd.After(time.Now())
}

// scheduledFor is the subject's card plan waiting to start, nil when none waits.
// A read that fails answers nil, as billingSubscription does: the one-plan rule
// is then held by the sale lease and the stable processor key.
func scheduledFor(db *datastore.Datastore, subject string, test bool) *subscription.Subscription {
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil
	}
	for _, s := range subs {
		if s.Status == subscription.Scheduled && !strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") {
			return s
		}
	}
	return nil
}

// schedule shapes a freshly started row to open at start on the card it names:
// nothing is served or charged before then, and the period it owes first begins
// there (engine.IsDue).
func schedule(sub *subscription.Subscription, start time.Time) {
	sub.Status = subscription.Scheduled
	sub.TrialStart, sub.TrialEnd = time.Time{}, time.Time{}
	sub.Start, sub.PeriodStart, sub.PeriodEnd = start, start, start
}

// saleTerms is what a priced sale decided, carried into the act that opens it.
type saleTerms struct {
	planID, slug        string
	qty                 int
	lines               []subscription.Line
	seats, charge, list int64
	cur                 currency.Type
	promoPercent        int
	promoName           string
	customer            int64
}

// scheduleSale opens the card plan a subject holding a plan that will not renew
// by card bought: SCHEDULED at starts, the held plan's end, on the card pm, with
// nothing charged now. The held plan is set to end at starts; were that to fail,
// the new row is withdrawn and nothing stands but the card on file.
func scheduleSale(ctx context.Context, db *datastore.Datastore, org *organization.Organization, in SubscribeIn, rec *idempotencykey.IdempotencyKey, p *plan.Plan, pm *paymentmethod.PaymentMethod, held *subscription.Subscription, starts time.Time, t saleTerms, unvault, abandon func()) (*saleOutcome, error) {
	if !pm.IsDefault {
		pm.IsDefault = true
		if err := pm.Update(); err != nil {
			log.Error("subscribe: failed to mark method %s default (subject=%s): %v", pm.Id(), in.Subject, err)
		}
	}
	sub, err := createSubscription(db, p, &createSubscriptionRequest{
		UserId:               in.Subject,
		PlanId:               t.planID,
		StoreId:              in.StoreID,
		DefaultPaymentMethod: pm.Id(),
		Quantity:             t.qty,
		Metadata:             map[string]interface{}{"source": "subscribe/card", "follows": held.Id()},
		Test:                 org.TestMode(),
		priced:               t.lines,
		starts:               starts,
		provider:             string(processor.Square),
		promoPercent:         t.promoPercent,
		promoName:            t.promoName,
		customerCents:        t.customer,
	})
	if err != nil {
		unvault()
		abandon()
		return nil, err
	}
	// The held plan ends where the card plan starts. A fresh read, since a row read
	// by query is not bound to the store.
	prior := subscription.New(db)
	if err := prior.GetById(held.Id()); err == nil {
		prior.EndCancel = true
		prior.CanceledAt = time.Now()
		err = prior.Update()
	}
	if err != nil {
		withdraw(db, sub)
		unvault()
		abandon()
		return nil, fmt.Errorf("subscribe: subscription %s could not be set to end at %s, so the card plan was not opened: %w",
			held.Id(), starts.UTC().Format(time.RFC3339), err)
	}
	at := starts
	sale := &Sale{
		SubscriptionID:  sub.Id(),
		PlanID:          t.planID,
		Level:           in.Level,
		Interval:        p.Interval,
		PaymentMethodID: pm.Id(),
		Seats:           t.seats,
		Category:        p.Category,
		Currency:        string(t.cur),
		Status:          "ok",
		StartsAt:        &at,
		StartsCents:     t.charge,
		Follows:         held.Id(),
	}
	if rec != nil {
		if body, mErr := json.Marshal(sale); mErr == nil {
			_ = idempotencykey.Complete(rec, string(body))
		}
	}
	noticeSale(ctx, saleNote{
		Org: org.Name, Email: in.Email, Plan: p, Seats: t.seats, Sale: sale,
		ListCents: t.list, PromoName: t.promoName, Test: org.TestMode(),
	})
	return &saleOutcome{Sale: sale, Sub: sub}, nil
}
