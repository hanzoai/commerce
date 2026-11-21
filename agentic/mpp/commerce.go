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
	"github.com/hanzoai/commerce/models/checkoutsession"
	"github.com/hanzoai/commerce/models/organization"
	orgpkg "github.com/hanzoai/commerce/pkg/org"
)

// usd is the currency of api/billing's agent checkout.
const usd = "usd"

// Commerce is the Seller that charges a shared payment token through api/billing's agent
// checkout. The checkout sells USD credit, so Commerce then spends the quoted amount from
// that credit through the credit ledger (creditledger.Set), keyed by the challenge id; the
// buyer's balance ends where it began. It offers nothing to a caller without an Org and a
// Subject, and nothing when there is neither an injected ledger nor a Spend. The receipt
// reference is the order's payment id.
type Commerce struct {
	// Account is the seller's Stripe profile for a caller.
	Account func(ctx context.Context, c Caller) (*Profile, error)
	// Store records the checkout a challenge opened, so every later attempt, on any
	// replica, completes that checkout under the same key: billing binds the charge to
	// checkout:<id>:<key>.
	Store Store
	// Orgs resolves the caller's Org; orgpkg.Resolve when nil.
	Orgs func(ctx context.Context, name string) (*organization.Organization, error)
	// Create and Complete are billing.CreateCheckout and billing.CompleteCheckout when nil.
	Create   func(ctx context.Context, org *organization.Organization, subject, host string, items []billing.ItemIn) (*billing.CheckoutView, *billing.PaymentFault)
	Complete func(ctx context.Context, org *organization.Organization, subject, host, id, key string, pay billing.PaymentIn) billing.CompleteOut
	// Spend draws cents from the subject's credit once per ref; nil draws through the
	// credit ledger.
	Spend func(ctx context.Context, org *organization.Organization, subject string, cents int64, ref string) error
}

var _ Seller = (*Commerce)(nil)

// Profile implements Seller.
func (c *Commerce) Profile(ctx context.Context, who Caller, q Quote) (*Profile, error) {
	if c.Store == nil {
		return nil, errors.New("commerce: Store is required")
	}
	if q.Currency != usd || who.Org == "" || who.Subject == "" || c.Account == nil || (c.Spend == nil && creditledger.Get() == nil) {
		return nil, ErrSkip
	}
	return c.Account(ctx, who)
}

// spend draws cents from the credit the checkout granted, at the address api/billing
// credited: the org's name as the ledger keys it.
func (c *Commerce) spend(ctx context.Context, org *organization.Organization, subject string, cents int64, ref string) error {
	if c.Spend != nil {
		return c.Spend(ctx, org, subject, cents, ref)
	}
	led := creditledger.Get()
	if led == nil {
		return errors.New("no credit ledger")
	}
	_, _, err := led.Debit(ctx, creditledger.DebitInput{
		Org: strings.ToLower(strings.TrimSpace(org.Name)), Subject: subject, Currency: usd,
		Reason: "mpp " + ref, Ref: ref, AmountCents: cents, Test: org.TestMode(),
	})
	return err
}

// checkout is the checkout of the sale's challenge: opened the first time, and the same
// one on every attempt after.
func (c *Commerce) checkout(ctx context.Context, org *organization.Organization, s Sale, cents int64) (string, error) {
	slot := "commerce/checkout/" + s.Challenge
	opened, found, err := c.Store.Load(ctx, slot)
	if err != nil || found {
		return string(opened), err
	}
	create := c.Create
	if create == nil {
		create = billing.CreateCheckout
	}
	view, fault := create(ctx, org, s.Caller.Subject, "", []billing.ItemIn{{Kind: checkoutsession.Credit, UsdCents: cents}})
	if fault != nil {
		return "", fmt.Errorf("checkout not created: %d %s", fault.Status, fault.Message)
	}
	if err := c.Store.Save(ctx, slot, []byte(view.ID), s.Until); err != nil {
		return "", fmt.Errorf("checkout %s opened and not recorded: %w", view.ID, err)
	}
	return view.ID, nil
}

// Take implements Seller: it completes the checkout on the token, then spends the credit.
// A refusal the checkout states is a *Problem; anything else is an outcome not known.
func (c *Commerce) Take(ctx context.Context, s Sale) (string, error) {
	if s.Currency != usd {
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
	id, err := c.checkout(ctx, org, s, cents)
	if err != nil {
		return "", fmt.Errorf("commerce: %w", err)
	}
	complete := c.Complete
	if complete == nil {
		complete = billing.CompleteCheckout
	}
	out := complete(ctx, org, s.Caller.Subject, "", id, s.Challenge, billing.PaymentIn{Type: "shared_payment_token", Token: s.Token})
	switch {
	case out.Status == 200 && out.View != nil && out.View.Order != nil:
		if err := c.spend(ctx, org, s.Caller.Subject, cents, s.Challenge); err != nil {
			return "", fmt.Errorf("commerce: %d cents charged and credited and not spent on challenge %s: %w", cents, s.Challenge, err)
		}
		if o := out.View.Order; o.PaymentID != "" {
			return o.PaymentID, nil
		}
		return out.View.Order.ID, nil
	case out.Status == 402 && out.View != nil && out.View.Error != nil:
		return "", refusal(out.View.Error)
	}
	return "", fmt.Errorf("commerce: checkout %s is not complete: status %d", id, out.Status)
}

// refusal is the problem for a refusal api/billing states.
func refusal(e *billing.AgentError) *Problem {
	switch e.Code {
	case "authentication_required":
		return Refuse(Action, e.Message)
	case "token_over_limit":
		return Refuse(Insufficient, e.Message)
	case "token_expired":
		return Refuse(Expired, e.Message)
	case "payments_unavailable":
		return Refuse(Internal, e.Message)
	}
	return Refuse(Failed, e.Message)
}
