// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package mpp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/billing/creditledger"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
)

// Commerce is the Seller that settles the stripe method through commerce's headless
// checkout, api/billing: a shared payment token is charged by CompleteCheckout, with the
// same grant checks, risk, refund and dispute paths as any agent checkout.
//
// What the money does there. A checkout sells a line of USD credit, and completing it
// charges the token and credits the buyer's balance by the amount. An MPP charge pays
// for a resource, not for credit, so in the same step Commerce spends exactly the
// resource's price, the amount the host quoted and the challenge carried, from the credit
// the checkout just granted, through the credit ledger the host injects with
// creditledger.Set (the account api/billing credits and its AI gate and balance read).
// The buyer's balance ends where it began, and the merchant is paid by the card charge.
// The spend is keyed by the challenge id, so it draws at most once. If the spend fails
// the resource is not served and the credit stays in the buyer's balance.
//
// Without an injected ledger and without a Spend of the host's, Commerce offers nothing:
// outside api/billing the credit cannot be spent, and charging a token for credit that
// buys the resource nothing would give the credit away. (api/billing keeps its own
// ledger's debit unexported.)
//
// The checkout needs a signed-in Subject and an Org, so Commerce offers nothing to an
// anonymous caller. Amounts are bounded by the checkout's own top-up floor and ceiling,
// which this package does not know; a price outside them fails at settlement as the
// host's error, not the buyer's.
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
	// Spend draws cents from the subject's credit for the act named by ref, all or
	// nothing and at most once per ref. Nil draws through the credit ledger the host
	// injected with creditledger.Set.
	Spend func(ctx context.Context, org *organization.Organization, subject string, cents int64, ref string) error
}

// errLedger says there is no ledger to spend the credit from.
var errLedger = errors.New("commerce: no credit ledger to spend the credit from")

// spend draws the price of the resource from the credit the checkout granted: at the
// address api/billing credited, the org's name as the ledger keys it, under the challenge
// id so a repeat draws nothing more.
func (c *Commerce) spend(ctx context.Context, org *organization.Organization, subject string, cents int64, ref string) error {
	if c.Spend != nil {
		return c.Spend(ctx, org, subject, cents, ref)
	}
	led := creditledger.Get()
	if led == nil {
		return errLedger
	}
	_, _, err := led.Debit(ctx, creditledger.DebitInput{
		Org: strings.ToLower(strings.TrimSpace(org.Name)), Subject: subject, Currency: "usd",
		Reason: "mpp " + ref, Ref: ref, AmountCents: cents, Test: org.TestMode(),
	})
	return err
}

var _ Seller = (*Commerce)(nil)

// Profile implements Seller. It offers USD to a caller with an Org and a Subject, when
// the credit the checkout grants can be spent on the resource.
func (c *Commerce) Profile(ctx context.Context, who Caller, q Quote) (*Profile, error) {
	if q.Currency != "usd" || who.Org == "" || who.Subject == "" || c.Account == nil {
		return nil, ErrSkip
	}
	if c.Spend == nil && creditledger.Get() == nil {
		return nil, ErrSkip
	}
	return c.Account(ctx, who)
}

// Take implements Seller. The checkout is created for the quoted cents of credit and
// completed on the token, under the challenge id as the Idempotency-Key that binds the
// charge to the checkout, and the resource's price is then spent from the credit.
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
		// The token was charged and the credit posted. Spend the resource's price from it
		// before anything is served; if that fails the credit stays with the buyer, and
		// the host is told, since the payment is neither refunded nor used.
		if err := c.spend(ctx, org, s.Caller.Subject, cents, s.Challenge); err != nil {
			return "", fmt.Errorf("commerce: %d cents were charged and credited and not spent on challenge %s: %w", cents, s.Challenge, err)
		}
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
