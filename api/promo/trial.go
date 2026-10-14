// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package promo

// A plan's free trial is an offer each card takes once, whichever org buys the
// plan. Buyers' subscriptions live in their own orgs' namespaces, so the record
// that spans them lives with the platform's other per-card offer uses: a taken
// trial is a models/couponredemption row in the platform namespace, keyed by
// (the plan's trial, the card), written under its offer's lock so two buyers
// racing with one card cannot both take it.

import (
	"context"
	"errors"
	"strings"

	"github.com/hanzoai/commerce/models/couponredemption"
)

// ErrTrialTaken is a trial refused because the card has already had it.
var ErrTrialTaken = errors.New("this card has already had this plan's free trial")

// trialCode is the offer a plan's trial is recorded as. The colon keeps it apart
// from every coupon code, which CouponCode never admits one into. A sandbox trial
// is its own offer, so a test card never spends a live trial.
func trialCode(slug string, test bool) string {
	if test {
		return "trial-test:" + slug
	}
	return "trial:" + slug
}

// TakeTrial records that the card instrument takes plan slug's free trial, or
// answers ErrTrialTaken when that card has had it. instrument is the card's
// stable identity, as Reserve takes it. release gives the trial back, for a sale
// that took it and then did not open it.
func TakeTrial(ctx context.Context, slug, instrument string, test bool) (release func(), err error) {
	slug, instrument = strings.TrimSpace(slug), strings.TrimSpace(instrument)
	if slug == "" || instrument == "" {
		return nil, errors.New("trial: a trial needs a plan and a card")
	}
	code := trialCode(slug, test)
	card := cardRedeemer(instrument)
	id := couponredemption.DeterministicID(code, card)

	mu := couponLock(code)
	mu.Lock()
	defer mu.Unlock()
	db := platform(ctx)
	if ok, err := redemptionExists(db, id); err != nil {
		return nil, err
	} else if ok {
		return nil, ErrTrialTaken
	}
	if err := putRedemption(db, id, code, card); err != nil {
		return nil, err
	}
	// The release must still run when the buyer has hung up, so it keeps the
	// context's values and drops its cancellation.
	keep := context.WithoutCancel(ctx)
	return func() {
		mu.Lock()
		defer mu.Unlock()
		removeRedemption(platform(keep), code, id)
	}, nil
}
