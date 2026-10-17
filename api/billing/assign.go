// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"context"
	"fmt"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/json/http"
)

// AssignSeats sets which member holds which of a team's seat types, replacing
// what was assigned before, and answers with the row as it now stands. holder is
// as CancelSubscription's: AnyHolder for the org's own authority, else the wallet
// the caller pays from, which holds no team's subscription.
//
// Each member is a person of the org ("<org>/<name>"), named once, on a seat
// type the subscription holds seats of, and no seat type is given more members
// than the seats of it paid for now: the fewer of the period in hand's and the
// next one's, as members are counted on a change of seats. An empty list
// unassigns every seat. Which member holds a seat moves no money and is not a
// usage bound: a team's usage is its seats' together.
func AssignSeats(ctx context.Context, org *organization.Organization, holder, id string, seats []subscription.Seat) (*Subscription, error) {
	sub, err := assignSeats(ctx, org, holder, id, seats)
	if err != nil {
		return nil, err
	}
	return viewSubscription(sub), nil
}

// assignSeats is the worker, returning the ROW so the endpoint renders its body
// off the model, as cancelSubscription does. It writes only while the row is the
// one it read, and reads again when a renewal or a change of seats wrote first,
// so an assignment never puts back what another writer moved.
func assignSeats(ctx context.Context, org *organization.Organization, holder, id string, seats []subscription.Seat) (*subscription.Subscription, error) {
	for range mixin.Swaps {
		sub, err := heldSubscription(ctx, org, holder, id)
		if err != nil {
			return nil, err
		}
		out, err := assigned(org.Name, sub, seats)
		if err != nil {
			return nil, err
		}
		sub.Assigned = out
		ok, err := sub.Swap()
		if err != nil {
			return nil, err
		}
		if ok {
			return sub, nil
		}
	}
	return nil, fmt.Errorf("subscription %s changed under %d reads in a row", id, mixin.Swaps)
}

// assigned is seats as the row holds them, or why they cannot be: each names a
// person of the org (member), once, on a seat type the row holds, and no seat type
// is given more members than the seats of it paid for now.
func assigned(org string, sub *subscription.Subscription, seats []subscription.Seat) ([]subscription.Seat, error) {
	slug := subscriptionSlug(sub)
	if !perSeat(slug) {
		return nil, subValidationError{fmt.Sprintf("plan %q has no seats to assign", slug)}
	}
	room := map[string]int{slug: paidNow(sub.Quantity, sub.PendingQuantity)}
	for _, l := range sub.Lines {
		room[l.Plan] = paidNow(l.Quantity, l.PendingQuantity)
	}
	seen, used := map[string]bool{}, map[string]int{}
	out := make([]subscription.Seat, 0, len(seats))
	for _, s := range seats {
		who, plan := member(org, s.Member), strings.TrimSpace(s.Plan)
		switch n, ok := room[plan]; {
		case who == "":
			return nil, subValidationError{fmt.Sprintf("member %q is not a person of %s: name them %s/<name>", strings.TrimSpace(s.Member), org, org)}
		case seen[who]:
			return nil, subValidationError{fmt.Sprintf("member %q is given two seats", who)}
		case !ok:
			return nil, subValidationError{fmt.Sprintf("seat type %q is not on this subscription", plan)}
		case used[plan] == n:
			return nil, subValidationError{fmt.Sprintf("seat type %q has %d seats paid for", plan, n)}
		}
		seen[who] = true
		used[plan]++
		out = append(out, subscription.Seat{Member: who, Plan: plan})
	}
	return out, nil
}

// member is the person a seat names, as the org's own accounts name a person —
// "<org>/<name>", folded to lower case — or "" when it names no person of org: a
// name in another org, a bare name, or a path deeper than one person.
func member(org, s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	owner, name, ok := strings.Cut(s, "/")
	if !ok || name == "" || strings.Contains(name, "/") || owner != strings.ToLower(strings.TrimSpace(org)) {
		return ""
	}
	return s
}

// paidNow is the seats of a seat type paid for now: the period in hand's, or the
// next period's when a decrease waits for it.
func paidNow(quantity, pending int) int {
	if pending > 0 {
		return min(quantity, pending)
	}
	return quantity
}

type assignSeatsRequest struct {
	Assigned []subscription.Seat `json:"assigned"`
}

// AssignBillingSeats sets which member holds which of a team's seat types.
//
//	PUT /v1/billing/subscriptions/:id/seats   { "assigned": [{ "member", "plan" }] }
func AssignBillingSeats(c *zip.Ctx) error {
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "sign in to change your subscription", nil)
	}
	// A team's seats are its admins' to assign: a member paying from the org's
	// pool holds the subscription as its wallet does, and still assigns nothing.
	if !mayActForOrg(c) {
		return http.Fail(c, 403, "only an admin of the org assigns its seats", nil)
	}
	var req assignSeatsRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	sub, err := assignSeats(c.Context(), org, callerHolder(c), c.Param("id"), req.Assigned)
	if err != nil {
		switch {
		case IsSubscriptionNotFound(err):
			return http.Fail(c, 404, "subscription not found", err)
		case IsSubscriptionRefused(err):
			return http.Fail(c, 400, err.Error(), nil)
		}
		log.Error("Failed to assign seats: %v", err, c)
		return http.Fail(c, 500, "failed to assign seats", err)
	}
	return c.JSON(200, subscriptionResponse(sub))
}
