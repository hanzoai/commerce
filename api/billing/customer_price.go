package billing

// customer_price.go — a price one customer pays for one plan, every period.
//
// Staff agree a price with a customer ("Unlimited at $195 a month, for good"):
// SetCustomerPrice records it for the subject, plan and period length, and puts it
// on the subject's live subscriptions of that plan. A card sale of the plan to the
// subject charges it (price), the subscription it opens carries it, and every
// renewal bills it: the invoice states the plan at its list price and the
// difference as the customer's discount (engine.CustomerPriceLabel). No promotion
// applies beside it. A price at or above list is refused; 0 removes the price, and
// the next renewal bills the list price again.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/customerprice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
)

// PriceIn is one customer's price for one plan.
type PriceIn struct {
	// Subject is the billing subject, already bounded to the org by the caller.
	Subject string
	// Plan is the plan's slug; Interval its period, "month" (the default) or "year".
	Plan     string
	Interval types.Interval
	// Cents is one seat's price for one period, below the plan's list price. 0
	// removes the price.
	Cents int64
	// Note is why, kept on the price.
	Note string
}

// CustomerPrice is a price as it stands.
type CustomerPrice struct {
	Subject  string `json:"subject"`
	Plan     string `json:"plan"`
	Interval string `json:"interval"`
	// Cents is the agreed price of one seat for one period; ListCents the plan's.
	Cents     int64  `json:"cents"`
	ListCents int64  `json:"listCents"`
	Note      string `json:"note,omitempty"`
}

// PriceSet is what setting a price did: the price as it now stands (Cents 0 when
// it was removed) and every subscription of the plan the subject holds, as each
// now bills.
type PriceSet struct {
	Price         CustomerPrice  `json:"price"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// intervalOf is a plan's period as a price is keyed by it.
func intervalOf(i types.Interval) string {
	if i == types.Yearly {
		return string(types.Yearly)
	}
	return string(types.Monthly)
}

// agreedPrice is the price staff agreed subject pays for one seat of slug at p's
// period; 0 when there is none, or when it is not below p's list price.
func agreedPrice(db *datastore.Datastore, subject, slug string, p *plan.Plan) (int64, error) {
	r := customerprice.New(db)
	err := r.Get(db.NewKey(r.Kind(), customerprice.ID(subject, slug, intervalOf(p.Interval)), 0, nil))
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return 0, nil
	case err != nil:
		return 0, err
	}
	if r.Cents <= 0 || r.Cents >= int64(p.Price) {
		return 0, nil
	}
	return r.Cents, nil
}

// SetCustomerPrice records what in.Subject pays for one seat of in.Plan each
// period and puts it on every live subscription of that plan the subject holds,
// so the next invoice of each bills it and the next payment recorded for one is
// checked against it. Refused, with nothing written, when the
// plan is not one staff may assign, when the price is not below the plan's list
// price, or when a sale of the subject is in flight.
func SetCustomerPrice(ctx context.Context, org *organization.Organization, in PriceIn) (*PriceSet, error) {
	if org == nil {
		return nil, errors.New("customer price: no organization")
	}
	subject, slug := strings.TrimSpace(in.Subject), strings.TrimSpace(in.Plan)
	switch {
	case subject == "":
		return nil, saleRefusal{saleRefused, "subject is required"}
	case slug == "":
		return nil, saleRefusal{saleRefused, "plan is required"}
	case in.Cents < 0:
		return nil, saleRefusal{saleRefused, "cents is 0 (no price of its own) or more"}
	}
	db := datastore.New(org.Namespaced(ctx))
	lease, err := holdSale(db, subject)
	if err != nil {
		return nil, err
	}
	defer lease.Release()

	p, err := resolveSubscriptionPlan(db, slug)
	if err != nil || !p.Assignable() {
		return nil, saleRefusal{saleMissing, fmt.Sprintf("plan %q not found", slug)}
	}
	if p.Slug != "" {
		slug = p.Slug
	}
	if p, err = planAtInterval(p, in.Interval); err != nil {
		return nil, saleRefusal{saleRefused, err.Error()}
	}
	list := int64(p.Price)
	if in.Cents > 0 && in.Cents >= list {
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"plan %q lists at %d cents a %s; a customer's price is below it (got %d)", slug, list, intervalOf(p.Interval), in.Cents)}
	}

	interval := intervalOf(p.Interval)
	r := customerprice.New(db)
	key := db.NewKey(r.Kind(), customerprice.ID(subject, slug, interval), 0, nil)
	gerr := r.Get(key)
	switch {
	case gerr != nil && !errors.Is(gerr, datastore.ErrNoSuchEntity):
		return nil, fmt.Errorf("customer price: %w", gerr)
	case in.Cents == 0 && gerr == nil:
		if err := r.Delete(); err != nil {
			return nil, fmt.Errorf("customer price: %w", err)
		}
	case in.Cents > 0:
		r.Subject, r.Plan, r.Interval, r.Cents, r.Note = subject, slug, interval, in.Cents, strings.TrimSpace(in.Note)
		if gerr == nil {
			err = r.Update()
		} else {
			r.SetId(customerprice.ID(subject, slug, interval))
			err = r.Create()
		}
		if err != nil {
			return nil, fmt.Errorf("customer price: %w", err)
		}
	}

	out := &PriceSet{
		Price:         CustomerPrice{Subject: subject, Plan: slug, Interval: interval, Cents: in.Cents, ListCents: list, Note: strings.TrimSpace(in.Note)},
		Subscriptions: []Subscription{},
	}
	subs, err := userSubscriptions(db, subject, org.TestMode())
	if err != nil {
		return nil, fmt.Errorf("customer price: the price was set and the subscriptions it applies to could not be read: %w", err)
	}
	for _, s := range subs {
		if !pricedBy(s, slug, p.Interval) {
			continue
		}
		row := subscription.New(db)
		if err := row.GetById(s.Id()); err != nil {
			return nil, fmt.Errorf("customer price: subscription %s: %w", s.Id(), err)
		}
		row.CustomerCents = in.Cents
		if err := row.Update(); err != nil {
			return nil, fmt.Errorf("customer price: subscription %s: %w", s.Id(), err)
		}
		out.Subscriptions = append(out.Subscriptions, *viewSubscription(row))
	}
	return out, nil
}

// pricedBy reports whether a customer's price for slug at interval applies to s: a
// live subscription of that plan. A card plan's next invoice bills it; a plan
// collected outside commerce checks the next payment recorded against it. A
// bundle row is not a purchase.
func pricedBy(s *subscription.Subscription, slug string, interval types.Interval) bool {
	switch s.Status {
	case subscription.Active, subscription.Trialing, subscription.PastDue, subscription.Scheduled:
	default:
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") &&
		subscriptionSlug(s) == slug && intervalOf(s.Plan.Interval) == intervalOf(interval)
}

// CustomerPrices is every price staff agreed with the org's subjects.
func CustomerPrices(ctx context.Context, org *organization.Organization) ([]CustomerPrice, error) {
	if org == nil {
		return nil, errors.New("customer prices: no organization")
	}
	db := datastore.New(org.Namespaced(ctx))
	rows := make([]*customerprice.CustomerPrice, 0)
	if _, err := customerprice.Query(db).GetAll(&rows); err != nil {
		return nil, err
	}
	out := make([]CustomerPrice, 0, len(rows))
	for _, r := range rows {
		list := int64(0)
		if p, err := resolveSubscriptionPlan(db, r.Plan); err == nil {
			if at, err := planAtInterval(p, types.Interval(r.Interval)); err == nil {
				list = int64(at.Price)
			}
		}
		out = append(out, CustomerPrice{Subject: r.Subject, Plan: r.Plan, Interval: r.Interval, Cents: r.Cents, ListCents: list, Note: r.Note})
	}
	return out, nil
}
