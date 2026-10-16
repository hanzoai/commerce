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
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/json/http"
)

// AssignSeats sets which member holds which of a team's seat types, replacing
// what was assigned before, and answers with the row as it now stands. holder is
// as CancelSubscription's: AnyHolder for the org's own authority, else the wallet
// the caller pays from, which holds no team's subscription.
//
// Each member is named once, on a seat type the subscription holds seats of, and
// no seat type is given more members than the seats of it paid for now: the
// fewer of the period in hand's and the next one's, as members are counted on a
// change of seats. An empty list unassigns every seat. Which member holds a seat
// moves no money and is not a usage bound: a team's usage is its seats' together.
func AssignSeats(ctx context.Context, org *organization.Organization, holder, id string, seats []subscription.Seat) (*Subscription, error) {
	sub, err := assignSeats(ctx, org, holder, id, seats)
	if err != nil {
		return nil, err
	}
	return viewSubscription(sub), nil
}

// assignSeats is the worker, returning the ROW so the endpoint renders its body
// off the model, as cancelSubscription does.
func assignSeats(ctx context.Context, org *organization.Organization, holder, id string, seats []subscription.Seat) (*subscription.Subscription, error) {
	sub, err := heldSubscription(ctx, org, holder, id)
	if err != nil {
		return nil, err
	}
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
		member, plan := strings.TrimSpace(s.Member), strings.TrimSpace(s.Plan)
		switch n, ok := room[plan]; {
		case member == "":
			return nil, subValidationError{"every seat names its member"}
		case seen[member]:
			return nil, subValidationError{fmt.Sprintf("member %q is given two seats", member)}
		case !ok:
			return nil, subValidationError{fmt.Sprintf("seat type %q is not on this subscription", plan)}
		case used[plan] == n:
			return nil, subValidationError{fmt.Sprintf("seat type %q has %d seats paid for", plan, n)}
		}
		seen[member] = true
		used[plan]++
		out = append(out, subscription.Seat{Member: member, Plan: plan})
	}
	sub.Assigned = out
	if err := sub.Update(); err != nil {
		return nil, err
	}
	return sub, nil
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
