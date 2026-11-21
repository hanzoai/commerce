// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Commerce is the Backend over commerce's headless purchase surface (api/billing: the
// catalog, CreateCheckout, ReadCheckout and CompleteCheckout). It prices nothing and moves
// no money itself: an item id is a catalog id (a credit pack, credit-<cents>, or a plan
// slug), the price is the catalog's, and the payment is the Stripe shared payment token the
// buyer granted the agent, charged by CompleteCheckout.
//
// Commerce sells exactly one unit of one item per checkout, in USD, and takes payment only by
// a shared payment token, which is a card payment. It refuses what that cannot honour.
type Commerce struct{}

var _ Backend = Commerce{}

// ErrSingle: commerce sells one unit of one item per checkout.
var ErrSingle = errors.New("ap2: commerce sells exactly one unit of one item per checkout")

// Open implements Backend.
func (c Commerce) Open(ctx context.Context, who Caller, items []Pick) (*Cart, error) {
	if len(items) != 1 || items[0].Quantity != 1 {
		return nil, ErrSingle
	}
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	in, line, err := sold(ctx, org, who.Host, items[0].ID)
	if err != nil {
		return nil, err
	}
	v, f := billing.CreateCheckout(ctx, org, who.Subject, who.Host, []billing.ItemIn{in})
	if f != nil {
		return nil, refused(f, true)
	}
	return cart(v, line), nil
}

// Read implements Backend. The lines are priced from the catalog as it is now, while the
// total is the checkout's own: they differ when the catalog has moved since the checkout was
// opened, and the merchant refuses a checkout in which they do.
func (c Commerce) Read(ctx context.Context, who Caller, id string) (*Cart, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f, false)
	}
	if len(v.Items) != 1 {
		return nil, fmt.Errorf("ap2: checkout %s does not hold exactly one item", id)
	}
	item := catalogID(v.Items[0])
	_, line, err := sold(ctx, org, who.Host, item)
	if errors.Is(err, ErrNoItem) {
		return nil, mandate(ErrStale, "%s is no longer sold", item)
	}
	if err != nil {
		return nil, err
	}
	return cart(v, line), nil
}

// Pay implements Backend. The charge is bound to the checkout and the key, so a completion
// sent twice is one purchase.
func (c Commerce) Pay(ctx context.Context, who Caller, id, key string, pay Payment) (*Paid, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	// A canceled checkout is refused here: completion would take it for one still open.
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f, false)
	}
	if v.Status == checkoutsession.Canceled {
		return nil, ErrNotFound
	}
	out := billing.CompleteCheckout(ctx, org, who.Subject, who.Host, id, key, billing.PaymentIn{Type: "shared_payment_token", Token: pay.Token})
	return paid(out)
}

// Honours implements Backend: commerce takes a shared payment token, which pays by card.
func (Commerce) Honours(_ context.Context, _ Caller, in Instrument) error {
	if !strings.EqualFold(in.Type, "card") {
		return &Unsupported{Instrument: in, Why: "commerce takes payment by card, as a shared payment token"}
	}
	return nil
}

func (Commerce) org(ctx context.Context, who Caller) (*organization.Organization, error) {
	org, err := orgpkg.Resolve(ctx, who.Org)
	switch {
	case errors.Is(err, orgpkg.ErrResolveFailed):
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	case err != nil:
		return nil, err
	}
	return org, nil
}

// sold resolves an item id to what commerce sells: a credit pack of the catalog by its id, or
// a plan by its slug. It reads the price from the catalog.
func sold(ctx context.Context, org *organization.Organization, host, item string) (billing.ItemIn, Line, error) {
	cat, err := billing.ReadCatalog(ctx, org, host)
	if err != nil {
		return billing.ItemIn{}, Line{}, fmt.Errorf("%w: the catalog: %v", ErrUnavailable, err)
	}
	for _, p := range cat.CreditPacks {
		if p.ID == item {
			return billing.ItemIn{Kind: checkoutsession.Credit, UsdCents: p.UsdCents},
				Line{ID: p.ID, Title: fmt.Sprintf("$%d.%02d USD credit", p.UsdCents/100, p.UsdCents%100), Price: p.UsdCents, Quantity: 1}, nil
		}
	}
	for _, p := range cat.Plans {
		if p.Slug == item {
			return billing.ItemIn{Kind: checkoutsession.Plan, Plan: p.Slug}, Line{ID: p.Slug, Title: p.Name, Price: p.Price, Quantity: 1}, nil
		}
	}
	return billing.ItemIn{}, Line{}, ErrNoItem
}

// catalogID is the catalog id of what a checkout line sells.
func catalogID(l billing.ItemIn) string {
	if l.Kind == checkoutsession.Plan {
		return l.Plan
	}
	return fmt.Sprintf("credit-%d", l.UsdCents)
}

func cart(v *billing.CheckoutView, line Line) *Cart {
	return &Cart{ID: v.ID, Currency: strings.ToUpper(v.Currency), Lines: []Line{line}, Total: v.TotalCents, Canceled: v.Status == checkoutsession.Canceled}
}

// refused states a failure of CreateCheckout or ReadCheckout. For an opening, a 4xx is a
// refusal of the item; otherwise it is a checkout that is not there for this caller.
func refused(f *billing.PaymentFault, opening bool) error {
	switch {
	case f.Status >= http.StatusInternalServerError:
		return fmt.Errorf("%w: %s", ErrUnavailable, f.Message)
	case opening:
		return fmt.Errorf("%w: %s", ErrNoItem, f.Message)
	}
	return ErrNotFound
}

// paid reads what CompleteCheckout answered. A paid checkout is an order; a payment the
// token or the bank refused is a *Declined; the rest is the store's state or its outage.
func paid(out billing.CompleteOut) (*Paid, error) {
	v := out.View
	if v == nil {
		return nil, errors.New("ap2: CompleteCheckout answered no checkout")
	}
	if out.Status >= http.StatusInternalServerError {
		msg := "billing is unavailable"
		if v.Error != nil {
			msg = v.Error.Message
		}
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, msg)
	}
	if v.Error == nil {
		if v.Status != checkoutsession.Complete || v.Order == nil {
			return nil, fmt.Errorf("ap2: CompleteCheckout answered %d with the checkout %s", out.Status, v.Status)
		}
		return &Paid{Order: v.Order.ID, Payment: v.Order.PaymentID}, nil
	}
	switch v.Error.Code {
	case "not_found":
		return nil, ErrNotFound
	case "unavailable", "stablecoin_unavailable":
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, v.Error.Message)
	case "already_completed":
		return nil, mandate(ErrSpent, "%s", v.Error.Message)
	case "in_progress":
		return nil, mandate(ErrBusy, "%s", v.Error.Message)
	case "idempotency_key_required", "payment_type_unknown":
		return nil, fmt.Errorf("ap2: CompleteCheckout refused the request it was given: %s", v.Error.Code)
	}
	return nil, &Declined{Code: v.Error.Code, Message: v.Error.Message}
}

// Durable is a Ledger kept in commerce's datastore (models/idempotencykey), so a claim holds
// across restarts and across every node.
type Durable struct{}

const ledgerScope = "ap2/open"

// Begin implements Ledger.
func (Durable) Begin(ctx context.Context, open, ref string) (Verdict, Lease, error) {
	db := datastore.New(nscontext.WithNamespace(ctx, "system"))
	rec, replay, err := idempotencykey.Begin(db, ledgerScope, open)
	if err != nil {
		return 0, nil, err
	}
	switch {
	case !replay:
		return Fresh, &durable{rec: rec, ref: ref}, nil
	case rec.Status != idempotencykey.StatusCompleted:
		return Busy, nil, nil
	case rec.Response == ref:
		return Replay, nil, nil
	}
	return Taken, nil, nil
}

type durable struct {
	rec *idempotencykey.IdempotencyKey
	ref string
}

func (d *durable) Keep(context.Context) error { return idempotencykey.Complete(d.rec, d.ref) }
func (d *durable) Drop(context.Context) error { return d.rec.Delete() }
