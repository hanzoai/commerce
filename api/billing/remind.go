package billing

// remind.go — telling a subscriber, before it ends, that their plan will not
// renew by card.
//
// Two kinds of plan end without a charge: one collected outside commerce (a wire,
// an invoice raised in the processor's dashboard, a comp), which lapses at its end
// unless the next payment is recorded, and one the balance used to renew, which is
// past due at its end since a plan's period is never paid from prepaid money. A
// week before the end and again a day before, each such plan's subscriber is told
// so, once per lead for each period, with the way to keep it: subscribe by card,
// which starts at that end and charges nothing until then (scheduleSale).
//
// Commerce decides who is due a reminder and keeps that each was sent; the host
// delivers it (send). A send that fails gives its claim back, so the next sweep
// tries again.

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
)

// RemindLeads are how long before a period ends its subscriber is reminded,
// longest first.
var RemindLeads = []time.Duration{7 * 24 * time.Hour, 24 * time.Hour}

// The kinds of plan a reminder is about.
const (
	RemindComp     = "comp"     // a plan given away
	RemindExternal = "external" // a plan paid outside checkout
	RemindPrepaid  = "prepaid"  // a plan the balance used to renew
)

// Reminder is one subscriber to tell that their plan ends without a charge.
type Reminder struct {
	Org            string
	Subject        string
	SubscriptionID string
	// Plan is the plan's slug, PlanName its name, Interval its period.
	Plan     string
	PlanName string
	Interval string
	// Ends is when the period ends; Lead which reminder this is.
	Ends time.Time
	Lead time.Duration
	// Kind is RemindComp, RemindExternal or RemindPrepaid.
	Kind string
}

// RemindRun is one pass: how many reminders were due, how many were sent, and
// how many failed and will be tried again.
type RemindRun struct {
	Due    int `json:"due"`
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

// RemindDue sends every reminder due at now, in every org, through send: each
// plan that ends without a charge within a lead, once per lead for each period.
// A subscriber already holding a card plan scheduled to follow is not reminded.
// Only the population read failing ends the pass.
func RemindDue(ctx context.Context, now time.Time, send func(context.Context, Reminder) error) (*RemindRun, error) {
	orgs := make([]*organization.Organization, 0)
	if _, err := organization.Query(datastore.New(ctx)).GetAll(&orgs); err != nil {
		return nil, err
	}
	run := &RemindRun{}
	for _, org := range orgs {
		remindOrg(ctx, org, now, send, run)
	}
	return run, nil
}

// remindOrg sends the reminders due at now in one org, counting them on run.
func remindOrg(ctx context.Context, org *organization.Organization, now time.Time, send func(context.Context, Reminder) error, run *RemindRun) {
	subs, err := ListSubscriptions(ctx, org, "", "")
	if err != nil {
		log.Error("Reminders: subscriptions of org %q could not be read: %v", org.Name, err)
		return
	}
	db := datastore.New(org.Namespaced(ctx))
	for _, s := range subs {
		r, ok := reminderFor(org, s, now)
		if !ok || scheduledFor(db, s.UserId, s.Test) != nil {
			continue
		}
		rec, replay, err := idempotencykey.Begin(db, "billing-remind:"+s.Id(), strconv.FormatInt(r.Ends.Unix(), 10)+":"+r.Lead.String())
		switch {
		case err != nil:
			log.Error("Reminders: the reminder for subscription %s could not be claimed: %v", s.Id(), err)
			run.Due++
			run.Failed++
			continue
		case replay:
			continue
		}
		run.Due++
		if err := send(ctx, r); err != nil {
			log.Error("Reminders: the reminder for subscription %s (org %q) was not sent: %v", s.Id(), org.Name, err)
			_ = rec.Delete()
			run.Failed++
			continue
		}
		_ = idempotencykey.Complete(rec, "sent")
		run.Sent++
	}
}

// reminderFor is the reminder s is due at now, if any: a live plan in its org's
// mode that ends without a charge within a lead, at the shortest lead it is
// within.
func reminderFor(org *organization.Organization, s *subscription.Subscription, now time.Time) (Reminder, bool) {
	if s.Test != org.TestMode() || s.Status != subscription.Active || s.EndCancel ||
		strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") {
		return Reminder{}, false
	}
	kind := ""
	switch {
	case s.Type == subscription.External && strings.EqualFold(strings.TrimSpace(s.ProviderType), processorComp):
		kind = RemindComp
	case s.Type == subscription.External:
		kind = RemindExternal
	case prepaidRenewed(s):
		kind = RemindPrepaid
	default:
		return Reminder{}, false
	}
	left := s.PeriodEnd.Sub(now)
	if left <= 0 {
		return Reminder{}, false
	}
	lead := time.Duration(0)
	for _, l := range RemindLeads {
		if left <= l {
			lead = l
		}
	}
	if lead == 0 {
		return Reminder{}, false
	}
	return Reminder{
		Org: org.Name, Subject: s.UserId, SubscriptionID: s.Id(),
		Plan: subscriptionSlug(s), PlanName: s.Plan.Name, Interval: intervalOf(s.Plan.Interval),
		Ends: s.PeriodEnd.UTC(), Lead: lead, Kind: kind,
	}, true
}
