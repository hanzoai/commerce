// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

// nextcoupon.go — a percent off a subscription's NEXT renewal invoice, put on by
// staff.
//
// It is not a plan coupon (api/promo): nobody types it at checkout and it has no
// window, limit or redemptions. It is one fact on the subject's live
// subscription row (subscription.NextCouponCode, NextCouponPercent), and the
// renewal that bills the next period takes it off that invoice's plan fee and
// clears it (engine.RenewSubscription). Setting it moves no money.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
)

// SetNextCoupon puts code at percent off the next renewal invoice of subject's
// live subscription — its one paid plan, or a trial of one — and answers with
// the row as it now stands. Percent 0 takes a waiting coupon off; 1 to 100 puts
// one on, replacing any that waits, and needs a code (promo.CouponCode: 1-64 of
// A-Z, 0-9, '-' and '_', stored upper case).
//
// A percent outside 0..100, a missing or malformed code, and a plan collected
// outside commerce, which no renewal ever invoices, are refused
// (IsSubscriptionRefused). A subject holding no live subscription is not found
// (IsSubscriptionNotFound).
func SetNextCoupon(ctx context.Context, org *organization.Organization, subject, code string, percent int) (*Subscription, error) {
	if org == nil {
		return nil, errors.New("next coupon: no organization")
	}
	subject = strings.TrimSpace(subject)
	switch {
	case subject == "":
		return nil, subValidationError{"subject is required"}
	case percent < 0 || percent > 100:
		return nil, subValidationError{fmt.Sprintf("percent must be 0 to clear or 1 to 100, got %d", percent)}
	}
	if percent > 0 {
		if code = promo.CouponCode(code); code == "" {
			return nil, subValidationError{"code is required: 1-64 of A-Z, 0-9, '-' and '_'"}
		}
	} else {
		code = ""
	}

	held := renewing(datastore.New(org.Namespaced(ctx)), subject, org.TestMode())
	if held == nil {
		return nil, fmt.Errorf("%w: %s holds no live subscription", errSubscriptionNotFound, subject)
	}
	if percent > 0 && held.Type == subscription.External {
		return nil, subValidationError{fmt.Sprintf(
			"subscription %s is collected outside commerce, so no renewal invoices it", held.Id())}
	}
	// A row read by query is not bound to the store, so the write goes through a
	// fresh read of the same id.
	sub, err := loadSubscription(ctx, org, held.Id())
	if err != nil {
		return nil, err
	}
	sub.NextCouponCode, sub.NextCouponPercent = code, percent
	if err := sub.Update(); err != nil {
		return nil, fmt.Errorf("next coupon: subscription %s: %w", sub.Id(), err)
	}
	return viewSubscription(sub), nil
}

// renewing is the live subscription a renewal bills next: the subject's one paid
// plan (holdsPlan), or else a trial of a paid plan not yet paid for, which bills
// its first paid period at its end once a card is on it. nil when it holds
// neither.
func renewing(db *datastore.Datastore, subject string, test bool) *subscription.Subscription {
	if held := billingSubscription(db, subject, test); held != nil {
		return held
	}
	subs, err := userSubscriptions(db, subject, test)
	if err != nil {
		return nil
	}
	for _, s := range subs {
		if s.Status == subscription.Trialing && !strings.EqualFold(strings.TrimSpace(s.ProviderType), "bundle") && paidRow(s) {
			return s
		}
	}
	return nil
}
