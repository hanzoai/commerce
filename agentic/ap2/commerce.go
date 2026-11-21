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
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
	"github.com/hanzoai/commerce/util/nscontext"
)

// Commerce is the Backend over commerce's headless purchase surface (api/billing): an item id is
// a catalog id, the price is the catalog's, and the payment is the Stripe shared payment token the
// buyer granted the agent, charged by CompleteCheckout. It sells one unit of one credit pack per
// checkout, in USD, by card; a plan (it would renew with no mandate), a second line and another
// instrument are refused.
type Commerce struct{}

var _ Backend = Commerce{}

// Read implements Backend. The lines are priced from the catalog as it is now and the total is the
// checkout's own: they differ once the catalog has moved since the checkout was opened.
func (c Commerce) Read(ctx context.Context, who Caller, id string) (*Cart, error) {
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	v, f := billing.ReadCheckout(ctx, org, who.Subject, id)
	if f != nil {
		return nil, refused(f)
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
	return &Cart{
		ID: v.ID, Currency: strings.ToUpper(v.Currency), Lines: []Line{line}, Total: v.TotalCents,
		Canceled: v.Status == checkoutsession.Canceled, Pending: v.Status == checkoutsession.Pending, Paid: v.Status == checkoutsession.Complete,
	}, nil
}

// Pay implements Backend. CompleteCheckout takes the checkout only if it is still the quoted line
// at the quoted total, and binds the charge to the checkout and the key.
func (c Commerce) Pay(ctx context.Context, who Caller, id, key string, pay Payment) (*Paid, error) {
	if len(pay.Cart.Lines) != 1 {
		return nil, ErrNoItem
	}
	org, err := c.org(ctx, who)
	if err != nil {
		return nil, err
	}
	in, _, err := sold(ctx, org, who.Host, pay.Cart.Lines[0].ID)
	if errors.Is(err, ErrNoItem) {
		return nil, mandate(ErrStale, "%s is no longer sold", pay.Cart.Lines[0].ID)
	}
	if err != nil {
		return nil, err
	}
	out := billing.CompleteCheckout(ctx, org, who.Subject, who.Host, id, key, billing.PaymentIn{
		Type: "shared_payment_token", Token: pay.Token,
		Quote: billing.Quote{Kind: in.Kind, UsdCents: in.UsdCents, Plan: in.Plan, TotalCents: pay.Cart.Total},
	})
	return paid(out)
}

// Honours implements Backend: a shared payment token pays by card, and commerce cannot tell which
// instrument it stands for, so it cannot honour a mandate that limits the instrument.
func (Commerce) Honours(_ context.Context, _ Caller, in Instrument, pinned bool) error {
	switch {
	case !strings.EqualFold(in.Type, "card"):
		return &Unsupported{Instrument: in, Why: "commerce takes payment by card, as a shared payment token"}
	case pinned:
		return &Unsupported{Instrument: in, Why: "commerce cannot tell which instrument a shared payment token stands for"}
	}
	return nil
}

func (Commerce) org(ctx context.Context, who Caller) (*organization.Organization, error) {
	org, err := orgpkg.Resolve(ctx, who.Org)
	switch {
	case errors.Is(err, orgpkg.ErrResolveFailed):
		return nil, fmt.Errorf("%w: the organization", ErrUnavailable)
	case err != nil:
		return nil, err
	}
	return org, nil
}

// sold resolves an item id to a credit pack of the catalog. A plan is refused: one payment mandate
// authorizes one payment, and a plan's first makes the token the subscription's payment method,
// charged at every renewal with no mandate.
func sold(ctx context.Context, org *organization.Organization, host, item string) (billing.ItemIn, Line, error) {
	cat, err := billing.ReadCatalog(ctx, org, host)
	if err != nil {
		return billing.ItemIn{}, Line{}, fmt.Errorf("%w: the catalog", ErrUnavailable)
	}
	for _, p := range cat.CreditPacks {
		if p.ID == item {
			return billing.ItemIn{Kind: checkoutsession.Credit, UsdCents: p.UsdCents},
				Line{ID: p.ID, Title: fmt.Sprintf("$%d.%02d USD credit", p.UsdCents/100, p.UsdCents%100), Price: p.UsdCents, Quantity: 1}, nil
		}
	}
	for _, p := range cat.Plans {
		if p.Slug == item {
			return billing.ItemIn{}, Line{}, &Unsupported{Item: item, Why: "a plan renews on its payment token with no mandate; only credit is sold through mandates"}
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

// refused states a failure of ReadCheckout: the store's outage, or a checkout that is not there
// for this caller.
func refused(f *billing.PaymentFault) error {
	if f.Status >= http.StatusInternalServerError {
		return fmt.Errorf("%w: %s", ErrUnavailable, f.Message)
	}
	return ErrNotFound
}

// paid reads what CompleteCheckout answered: a paid checkout is an order, a payment the token or
// the bank refused is a *Declined, and the rest is the store's state or its outage.
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
	case "checkout_changed":
		return nil, mandate(ErrStale, "%s", v.Error.Message)
	case "already_completed":
		return nil, mandate(ErrSpent, "%s", v.Error.Message)
	case "in_progress":
		return nil, mandate(ErrBusy, "%s", v.Error.Message)
	case "idempotency_key_required", "payment_type_unknown":
		return nil, fmt.Errorf("ap2: CompleteCheckout refused the request it was given: %s", v.Error.Code)
	}
	return nil, &Declined{Code: v.Error.Code, Message: v.Error.Message}
}

// Durable is a Ledger in commerce's datastore: a claim is a models/idempotencykey record, written
// directly because its Begin re-claims a guard left started for StartedTTL, which is wrong for a
// claim on money that may have moved. It is released only by Settle.
type Durable struct{}

const ledgerScope = "ap2/open"

func ledgerDB(ctx context.Context) *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(ctx, "system"))
}

// readClaim reads the record of an open mandate by its exact storage key, with the id it is
// stored under.
func readClaim(db *datastore.Datastore, open string) (*idempotencykey.IdempotencyKey, string, error) {
	rec := idempotencykey.New(db)
	id := idempotencykey.DeterministicID(ledgerScope, open)
	return rec, id, rec.Get(db.NewKey(rec.Kind(), id, 0, nil))
}

// holder reads a claim: the closed mandate and checkout that hold it, kept in Facts.
func holder(rec *idempotencykey.IdempotencyKey) Claim {
	ref, checkout, _ := strings.Cut(rec.Facts, "\x00")
	return Claim{Ref: ref, Checkout: checkout, At: rec.CreatedAt}
}

// Begin implements Ledger.
func (Durable) Begin(ctx context.Context, open, ref, checkout string) (Verdict, Claim, error) {
	db := ledgerDB(ctx)
	for range mixin.Rounds() {
		rec, id, err := readClaim(db, open)
		switch {
		case err == nil:
			claim := holder(rec)
			switch {
			case rec.Status != idempotencykey.StatusCompleted:
				return Busy, claim, nil
			case claim.Ref == ref:
				return Replay, claim, nil
			}
			return Taken, claim, nil
		case !errors.Is(err, datastore.ErrNoSuchEntity):
			return 0, Claim{}, err
		}
		rec = idempotencykey.New(db)
		rec.SetId(id)
		rec.Scope, rec.IdemKey, rec.Status, rec.Facts = ledgerScope, open, idempotencykey.StatusStarted, ref+"\x00"+checkout
		ok, err := rec.Swap()
		if err != nil {
			return 0, Claim{}, err
		}
		if ok {
			return Fresh, Claim{Ref: ref, Checkout: checkout, At: rec.CreatedAt}, nil
		}
	}
	return 0, Claim{}, errors.New("ap2: the claim on an open mandate changed under every read")
}

// Settle implements Ledger.
func (Durable) Settle(ctx context.Context, open, ref string, kept bool) error {
	rec, _, err := readClaim(ledgerDB(ctx), open)
	switch {
	case errors.Is(err, datastore.ErrNoSuchEntity):
		return nil
	case err != nil:
		return err
	case holder(rec).Ref != ref || rec.Status == idempotencykey.StatusCompleted:
		return nil
	case kept:
		return idempotencykey.Complete(rec, ref)
	}
	return rec.Delete()
}
