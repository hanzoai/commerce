// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
)

// Commerce is the Seller that settles the stripe method through commerce's headless
// checkout, api/billing: a shared payment token is charged by CompleteCheckout, with the
// same grant checks, risk, refund and dispute paths as any agent checkout.
//
// What the money does there. A checkout sells a line of USD credit, and completing it
// charges the token and credits the buyer's balance by the amount, so an MPP charge on
// commerce is the purchase of that much credit for the Caller's Subject: the host's
// resource then decides whether to spend it. The checkout needs a signed-in Subject and
// an Org, so Commerce offers nothing to an anonymous caller. Amounts are bounded by the
// checkout's own top-up floor and ceiling, which this package does not know; a price
// outside them fails at settlement as the host's error, not the buyer's.
//
// The receipt's reference is the ledger transaction the order carries as its payment id
// (the draft names the Stripe PaymentIntent, which the order does not expose).
type Commerce struct {
	// Account is the seller's Stripe side for a caller: the Business Network profile
	// and the payment method types the seller's account takes. It is the host's, since
	// a profile identifier is the account's own. A nil profile offers nothing.
	Account func(ctx context.Context, c Caller) (*Profile, error)
	// Orgs resolves the Caller's Org to the organization whose books are charged;
	// orgpkg.Resolve when nil.
	Orgs func(ctx context.Context, name string) (*organization.Organization, error)
	// Create and Complete are the checkout, billing.CreateCheckout and
	// billing.CompleteCheckout when nil.
	Create   func(ctx context.Context, org *organization.Organization, subject, host string, items []billing.ItemIn) (*billing.CheckoutView, *billing.PaymentFault)
	Complete func(ctx context.Context, org *organization.Organization, subject, host, id, key string, pay billing.PaymentIn) billing.CompleteOut
}

var _ Seller = (*Commerce)(nil)

// Profile implements Seller. It offers USD to a caller with an Org and a Subject.
func (c *Commerce) Profile(ctx context.Context, who Caller, q Quote) (*Profile, error) {
	if q.Currency != "usd" || who.Org == "" || who.Subject == "" || c.Account == nil {
		return nil, ErrSkip
	}
	return c.Account(ctx, who)
}

// Take implements Seller. The checkout is created for the quoted cents of credit and
// completed on the token, under the challenge id as the Idempotency-Key that binds the
// charge to the checkout.
//
// A refusal the checkout states is a *Problem: a token over its limit is Insufficient,
// an expired one Expired, a bank's further step Action, any other refused token or card
// Failed. Anything the checkout leaves unsaid, and a checkout that could not be made, is
// an error that is not a Problem: money may have moved, or the host must look.
func (c *Commerce) Take(ctx context.Context, s Sale) (string, error) {
	if s.Currency != "usd" {
		return "", Refuse(Unsupported, "Only US dollars are taken.")
	}
	cents, err := strconv.ParseInt(s.Amount, 10, 64)
	if err != nil {
		return "", Refuse(Bad, "The amount is not a whole number of cents.")
	}
	orgs := c.Orgs
	if orgs == nil {
		orgs = orgpkg.Resolve
	}
	org, err := orgs(ctx, s.Caller.Org)
	if err != nil || org == nil {
		return "", fmt.Errorf("commerce: organization %q: %v", s.Caller.Org, err)
	}
	create, complete := c.Create, c.Complete
	if create == nil {
		create = billing.CreateCheckout
	}
	if complete == nil {
		complete = billing.CompleteCheckout
	}
	view, fault := create(ctx, org, s.Caller.Subject, "", []billing.ItemIn{{Kind: "credit", UsdCents: cents}})
	if fault != nil {
		return "", fmt.Errorf("commerce: checkout not created: %d %s", fault.Status, fault.Message)
	}
	out := complete(ctx, org, s.Caller.Subject, "", view.ID, s.Challenge, billing.PaymentIn{Type: "shared_payment_token", Token: s.Token})
	if out.Status == 200 && out.View != nil && out.View.Order != nil {
		o := out.View.Order
		if o.PaymentID != "" {
			return o.PaymentID, nil
		}
		return o.ID, nil
	}
	if out.Status == 402 && out.View != nil && out.View.Error != nil {
		e := out.View.Error
		switch e.Code {
		case "authentication_required":
			return "", Refuse(Action, e.Message)
		case "token_over_limit":
			return "", Refuse(Insufficient, e.Message)
		case "token_expired":
			return "", Refuse(Expired, e.Message)
		case "payments_unavailable":
			return "", Refuse(Internal, e.Message)
		}
		return "", Refuse(Failed, e.Message)
	}
	return "", errors.New("commerce: checkout " + view.ID + " is not complete: status " + strconv.Itoa(out.Status))
}
