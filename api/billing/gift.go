package billing

// gift.go — days of a plan given, not sold, and the facts a referral asks of a
// subject's money before one is given.
//
// A referral earns its referrer a free week. Commerce owns the plan, so commerce
// gives it. A subject on a paid plan has the period it holds run that many days
// longer, so its next renewal comes that much later. A subject on no paid plan is
// given the entry plan (Pro) for that many days, on a row canceled at the end of
// them: it lapses back to Free on its own (engine.Over), with no card on file and
// nothing to collect, whether or not a renewal sweep ever runs.
//
// No money moves, no credit is minted and no invoice is raised. Each gift names a
// key and a key gives once: the platform retrying a gift whose answer it lost
// changes nothing, because the key is written on the row that received the days
// and is found there again.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/billing/trial"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/subscription"
)

// giftProvider marks a row nobody paid for: the provider billing/grant stamps
// on a comp, and the one a gift opens.
const giftProvider = "manual_gift"

// giftMost is the most days one gift may give.
const giftMost = 31

// The two things a gift can do.
const (
	GiftExtended = "extended" // the plan the subject holds runs the days longer
	GiftOpened   = "opened"   // the subject held none, and was given Pro for the days
)

// GiftIn is days of a plan to give one subject.
type GiftIn struct {
	// Subject is the billing subject the days are for, in the org the caller
	// names. This act never resolves an identity.
	Subject string
	// Days is how many, 1 to 31.
	Days int
	// Key names this gift. A key gives once per subject, ever.
	Key string
	// Reason is why, kept on the row beside the key.
	Reason string
	// Test gives the days in the sandbox books: a gift earned by a test-mode
	// payment speaks only to a test reader, so sandbox money never buys real time.
	Test bool
}

// Gifted is what a gift did.
type Gifted struct {
	// Outcome is GiftExtended or GiftOpened.
	Outcome string `json:"outcome"`
	// Subscription is the row that received the days, as it now stands.
	Subscription Subscription `json:"subscription"`
}

// Gift gives in.Days of a plan to in.Subject: the period of the paid plan it
// holds runs longer, or, holding none, it is given Pro for those days.
func Gift(ctx context.Context, org *organization.Organization, in GiftIn) (*Gifted, error) {
	if org == nil {
		return nil, errors.New("gift: no organization")
	}
	subject := strings.TrimSpace(in.Subject)
	key := strings.TrimSpace(in.Key)
	switch {
	case subject == "":
		return nil, saleRefusal{saleRefused, "subject is required"}
	case key == "":
		return nil, saleRefusal{saleRefused, "key is required: a gift gives once per key"}
	case in.Days < 1 || in.Days > giftMost:
		return nil, saleRefusal{saleRefused, fmt.Sprintf("days must be 1 to %d", giftMost)}
	}
	db := datastore.New(org.Namespaced(ctx))
	test := in.Test || org.TestMode()

	// Of two gifts racing on one key, one gives and the other replays. Without the
	// guard a retry could give twice, so a guard that cannot be taken refuses.
	rec, replay, err := idempotencykey.Begin(db, "billing-gift:"+subject, key)
	if err != nil {
		return nil, fmt.Errorf("gift: guard: %w", err)
	}
	if replay {
		if rec.Status == idempotencykey.StatusCompleted && rec.Response != "" {
			var prev Gifted
			if err := json.Unmarshal([]byte(rec.Response), &prev); err == nil {
				return &prev, nil
			}
		}
		return nil, saleRefusal{saleHeld, "this gift is being given now"}
	}
	out, err := give(ctx, org, db, subject, key, in, test)
	if err != nil {
		_ = rec.Delete()
		return nil, err
	}
	if body, err := json.Marshal(out); err == nil {
		_ = idempotencykey.Complete(rec, string(body))
	}
	return out, nil
}

// give is the act behind the guard. It reads the key off the subject's rows
// first: a guard re-claimed after a crash between the write and Complete finds
// its own gift there rather than giving it again.
func give(ctx context.Context, org *organization.Organization, db *datastore.Datastore, subject, key string, in GiftIn, test bool) (*Gifted, error) {
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil, fmt.Errorf("gift: read subscriptions: %w", err)
	}
	for _, s := range subs {
		if gave(s, key) {
			outcome := GiftExtended
			if s.ProviderType == giftProvider {
				outcome = GiftOpened
			}
			return &Gifted{Outcome: outcome, Subscription: *viewSubscription(s)}, nil
		}
	}

	now := time.Now().UTC()
	days := time.Duration(in.Days) * 24 * time.Hour
	note := map[string]interface{}{"key": key, "days": in.Days, "reason": in.Reason, "at": now.Format(time.RFC3339)}

	// The paid plan the subject holds, or else a gift still running, receives
	// the days, in the books the gift was earned in: sandbox time never lands on
	// a live row. A row read by query is not bound to the store, so the write goes
	// through a fresh read of the same id.
	var held *subscription.Subscription
	outcome := GiftExtended
	for _, s := range subs {
		if s.Test == test && s.Status == subscription.Active && holdsPlan(s) {
			held = s
			break
		}
	}
	if held == nil {
		held, outcome = running(subs, now, test), GiftOpened
	}
	if held != nil {
		sub, err := loadSubscription(ctx, org, held.Id())
		if err != nil {
			return nil, err
		}
		sub.PeriodEnd = later(sub.PeriodEnd, now).Add(days)
		remember(sub, note)
		if err := sub.Update(); err != nil {
			return nil, fmt.Errorf("gift: extend subscription %s: %w", sub.Id(), err)
		}
		return &Gifted{Outcome: outcome, Subscription: *viewSubscription(sub)}, nil
	}

	p, err := resolveSubscriptionPlan(db, trial.PlanSlug)
	if err != nil {
		return nil, saleRefusal{saleMissing, fmt.Sprintf("plan %q not found", trial.PlanSlug)}
	}
	if p, err = planAtInterval(p, ""); err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	q := *p
	q.TrialPeriodDays = 0
	sub := subscription.New(db)
	sub.UserId = subject
	sub.ProviderType = giftProvider
	sub.Quantity = 1
	sub.Test = test
	engine.StartSubscription(sub, &q)
	sub.Start, sub.PeriodStart, sub.PeriodEnd = now, now, now.Add(days)
	// Canceled at the end of its days from the first one: it lapses on its own,
	// and nothing ever renews or charges it.
	sub.EndCancel, sub.CanceledAt = true, now
	sub.Metadata = map[string]interface{}{"billing_reason": "gift"}
	remember(sub, note)
	if err := sub.Create(); err != nil {
		return nil, fmt.Errorf("gift: open subscription: %w", err)
	}
	return &Gifted{Outcome: GiftOpened, Subscription: *viewSubscription(sub)}, nil
}

// running is the subject's gift row in these books still serving at now, or nil.
func running(subs []*subscription.Subscription, now time.Time, test bool) *subscription.Subscription {
	for _, s := range subs {
		if s.Test == test && s.ProviderType == giftProvider && s.Status == subscription.Active && s.EndCancel && !engine.Over(s, now) {
			return s
		}
	}
	return nil
}

// later is the later of a and b.
func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// remember writes a gift's note on the row that received its days.
func remember(sub *subscription.Subscription, note map[string]interface{}) {
	if sub.Metadata == nil {
		sub.Metadata = map[string]interface{}{}
	}
	gifts, _ := sub.Metadata["gifts"].([]interface{})
	sub.Metadata["gifts"] = append(gifts, note)
}

// gave reports whether the row received the gift named key.
func gave(sub *subscription.Subscription, key string) bool {
	gifts, _ := sub.Metadata["gifts"].([]interface{})
	for _, g := range gifts {
		if m, ok := g.(map[string]interface{}); ok && m["key"] == key {
			return true
		}
	}
	return false
}

// Payment is a subscription period a card paid for in full.
type Payment struct {
	Invoice      string    `json:"invoice"`
	Subscription string    `json:"subscription"`
	Paid         time.Time `json:"paid"`
	// Cents is what the card paid; Refunded what has been given back against it.
	Cents    int64 `json:"cents"`
	Refunded int64 `json:"refunded"`
	// Disputed reports a dispute that withholds the payment's funds.
	Disputed bool `json:"disputed"`
	// Card is the processor's fingerprint of the card that paid: the same card
	// number reads the same in every vault. Empty when the row does not say.
	Card string `json:"card,omitempty"`
	// Test is a sandbox payment: no money moved.
	Test bool `json:"test"`
}

// Payments is what a referral asks of a subject's money.
type Payments struct {
	// First is the earliest subscription period a card paid for in full, or nil.
	// Prepaid credit and balance are not a card: a period they paid, wholly or in
	// part, is not one.
	First *Payment `json:"first,omitempty"`
	// Cards is the fingerprint of every card the subject keeps on file.
	Cards []string `json:"cards"`
}

// PaymentsOf answers [Payments] for one subject in the org.
func PaymentsOf(ctx context.Context, org *organization.Organization, subject string) (*Payments, error) {
	if org == nil {
		return nil, errors.New("payments: no organization")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, saleRefusal{saleRefused, "subject is required"}
	}
	db := datastore.New(org.Namespaced(ctx))

	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(db).Filter("UserId=", subject).GetAll(&invs); err != nil {
		return nil, fmt.Errorf("payments: read invoices: %w", err)
	}
	paid := invs[:0]
	for _, inv := range invs {
		if inv.Status == billinginvoice.Paid && inv.SubscriptionId != "" && inv.PaymentMethod == "card" && inv.AmountPaid > 0 && !inv.PaidAt.IsZero() {
			paid = append(paid, inv)
		}
	}
	sort.Slice(paid, func(i, j int) bool { return paid[i].PaidAt.Before(paid[j].PaidAt) })

	methods := make([]*paymentmethod.PaymentMethod, 0)
	if _, err := paymentmethod.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil)).Filter("CustomerId=", subject).GetAll(&methods); err != nil {
		return nil, fmt.Errorf("payments: read cards: %w", err)
	}
	prints := make(map[string]string, len(methods))
	out := &Payments{Cards: []string{}}
	for _, pm := range methods {
		if fp := metaStr(pm, "fingerprint"); fp != "" {
			prints[pm.Id()] = fp
			out.Cards = append(out.Cards, fp)
		}
	}

	if len(paid) > 0 {
		inv := paid[0]
		pay := &Payment{
			Invoice:      inv.Id(),
			Subscription: inv.SubscriptionId,
			Paid:         inv.PaidAt.UTC(),
			Cents:        inv.AmountPaid,
			Refunded:     inv.Refunded(),
			Disputed:     inv.Disputed(),
			Test:         org.TestMode(),
		}
		sub := subscription.New(db)
		if err := sub.GetById(inv.SubscriptionId); err == nil {
			pay.Test = pay.Test || sub.Test
			pay.Card = prints[strings.TrimSpace(sub.DefaultPaymentMethod)]
		}
		out.First = pay
	}
	return out, nil
}
