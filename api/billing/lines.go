// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	types "github.com/hanzoai/commerce/types"
)

// A team is sold in more than one seat type: the catalog's `group` names the
// plans that are seat types of one team (team, the standard seat, and
// team-premium). One subscription holds them all — its Plan and Quantity are the
// first seat type, and each other is a Line — so one charge pays the period, one
// invoice bills every seat type, one renewal renews them together, and the one
// row billingSubscription and the tier read is still the team's. The group's
// minSeats and maxMembers, which each of its plans states alike, count every
// seat: one standard and one premium seat is a team of two.

// LineIn is one more seat type a team buys beside its plan: a plan of the same
// catalog group, and how many seats of it.
type LineIn struct {
	PlanID   string `json:"planId"`
	Quantity int    `json:"quantity"`
}

// errLinePlan is a line naming a plan that is not on sale here.
var errLinePlan = errors.New("plan not found")

// linesAt prices a team's other seat types at the period the purchase chose, or
// says why they cannot be sold beside slug: each is a listed per-seat plan of
// slug's group, named once and never slug itself, with at least one seat, priced
// in cur. The line's price is the catalog's for that period, as the plan's own is
// (planAt).
// A line naming no such plan is errLinePlan; every other refusal is a
// subValidationError. No lines is no lines.
func linesAt(db *datastore.Datastore, slug string, in []LineIn, interval types.Interval, cur currency.Type) ([]subscription.Line, error) {
	if len(in) == 0 {
		return nil, nil
	}
	team := group(slug)
	if team == "" || !perSeat(slug) {
		return nil, subValidationError{fmt.Sprintf("plan %q is sold alone: it has no other seat types", slug)}
	}
	seen := map[string]bool{slug: true}
	out := make([]subscription.Line, 0, len(in))
	for _, l := range in {
		id := strings.TrimSpace(l.PlanID)
		lp, err := resolveSubscriptionPlan(db, id)
		if err != nil || !lp.Listed() {
			return nil, errLinePlan
		}
		if lp.Slug != "" {
			id = lp.Slug
		}
		if group(id) != team || !perSeat(id) {
			return nil, subValidationError{fmt.Sprintf("plan %q is not a seat type of %q", id, team)}
		}
		if seen[id] {
			return nil, subValidationError{fmt.Sprintf("seat type %q is named twice", id)}
		}
		seen[id] = true
		if lc := lp.Currency; lc != "" && cur != "" && lc != cur {
			return nil, subValidationError{fmt.Sprintf("seat type %q is priced in %s, not %s", id, lc, cur)}
		}
		if l.Quantity < 1 {
			return nil, subValidationError{fmt.Sprintf("seat type %q needs at least 1 seat (got %d)", id, l.Quantity)}
		}
		at, err := planAt(lp, 0, interval)
		if err != nil {
			return nil, subValidationError{err.Error()}
		}
		out = append(out, subscription.Line{Plan: id, Name: lp.Name, Price: at.Price, Quantity: l.Quantity})
	}
	return out, nil
}

// lineSeats is the seats the lines hold.
func lineSeats(lines []subscription.Line) int {
	n := 0
	for _, l := range lines {
		n += l.Quantity
	}
	return n
}

// lineCents is what the lines charge for one period: each one's price per seat
// times its seats.
func lineCents(lines []subscription.Line) int64 {
	var n int64
	for _, l := range lines {
		n += int64(l.Price) * int64(l.Quantity)
	}
	return n
}

// teamSeats is why a team of quantity seats of slug and the lines' seats cannot
// be sold, or "" when it can: a per-seat plan holds at least its group's minimum
// and at most its maximum, counted over every seat type.
func teamSeats(slug string, quantity int, lines []subscription.Line) string {
	team := quantity + lineSeats(lines)
	if why := overSeats(slug, team); why != "" {
		return why
	}
	if perSeat(slug) {
		if min := minSeats(slug); team < min {
			return fmt.Sprintf("plan %q requires at least %d seats (got %d)", slug, min, team)
		}
	}
	return ""
}

// everyLine reports whether covers holds for the plan and every line's plan: an
// offer — a promotion, a coupon — takes its percent off a team's charge only when
// it covers every seat type bought, since the charge and the invoice discount all
// of them together.
func everyLine(slug string, lines []subscription.Line, covers func(string) bool) bool {
	if !covers(slug) {
		return false
	}
	for _, l := range lines {
		if !covers(l.Plan) {
			return false
		}
	}
	return true
}

// nextLines is a team's other seat types as its next period opens them once a
// change asks for in: each seat type the row holds at the seats it already opens
// the next period at, or at the seats in asks for, and a seat type of the team's
// group the row does not hold yet added at its seats, priced as a sale prices it
// (linesAt) at the row's period and currency. paid is the seats of them paid for
// now: the fewer of the period in hand's and the next one's, which is none for a
// seat type added here, since the renewal that opens it is what charges it.
//
// An added seat type is billed in full (Line.Full), outside the promotion the
// row was sold under. One added and not yet charged is taken back by asking for
// 0 of it; a seat type the row has been charged for keeps at least one seat.
func nextLines(db *datastore.Datastore, sub *subscription.Subscription, slug string, in []LineIn) ([]subscription.Line, int, error) {
	next := slices.Clone(sub.Lines)
	for i := range next {
		if next[i].PendingQuantity > 0 {
			next[i].Quantity, next[i].PendingQuantity = next[i].PendingQuantity, 0
		}
	}
	cur := sub.Plan.Currency
	if cur == "" {
		cur = currency.USD
	}
	for _, l := range in {
		id := strings.TrimSpace(l.PlanID)
		i := slices.IndexFunc(next, func(n subscription.Line) bool { return n.Plan == id })
		if l.Quantity == 0 && i >= 0 && i < len(sub.Lines) && sub.Lines[i].Quantity == 0 {
			next[i].Quantity = 0
			continue
		}
		if l.Quantity < 1 {
			return nil, 0, subValidationError{fmt.Sprintf("seat type %q needs at least 1 seat (got %d)", id, l.Quantity)}
		}
		if i < 0 {
			added, err := linesAt(db, slug, []LineIn{{PlanID: id, Quantity: l.Quantity}}, sub.Plan.Interval, cur)
			if err != nil {
				return nil, 0, err
			}
			// linesAt names the plan by its slug: a line the row holds, asked for by
			// another of its ids, is that line and not a second one.
			if i = slices.IndexFunc(next, func(n subscription.Line) bool { return n.Plan == added[0].Plan }); i < 0 {
				line := added[0]
				line.Full = true
				next = append(next, line)
				continue
			}
		}
		next[i].Quantity = l.Quantity
	}
	paid := 0
	for i, l := range next {
		if i < len(sub.Lines) {
			paid += min(l.Quantity, sub.Lines[i].Quantity)
		}
	}
	return next, paid, nil
}

// pendLines sets the seats each of a team's other seat types opens the next
// period at (nextLines): the renewal that opens it charges them. A seat type
// added here holds no seats until then.
func pendLines(sub *subscription.Subscription, next []subscription.Line) {
	for i, n := range next {
		if i == len(sub.Lines) {
			sub.Lines = append(sub.Lines, subscription.Line{Plan: n.Plan, Name: n.Name, Price: n.Price, Full: n.Full})
		}
		l := &sub.Lines[i]
		if n.Quantity == l.Quantity {
			l.PendingQuantity = 0
		} else {
			l.PendingQuantity = n.Quantity
		}
	}
	// A seat type taken back before any period charged it holds nothing now and
	// opens nothing next: it is no longer the row's.
	sub.Lines = slices.DeleteFunc(sub.Lines, func(l subscription.Line) bool { return l.Quantity == 0 && l.PendingQuantity == 0 })
}

// compSeats is why a comp's seats are not changed: they are the ones its
// approval gave, and a record of the comp names them.
const compSeats = "a comp's seats are the ones its approval gave: record it again to change them"

// pendQuantity sets the seats of the plan's own seat type the next period opens
// at: quantity 0 leaves them as they are.
func pendQuantity(sub *subscription.Subscription, quantity int) {
	switch {
	case quantity <= 0:
	case quantity == sub.Quantity:
		sub.PendingQuantity = 0
	default:
		sub.PendingQuantity = quantity
	}
}

// ChangeSeats sets the seats a team opens its next period at: quantity of its
// plan's own seat type (0 keeps them), and each seat type lines names, a seat type
// of the team's group it does not hold yet among them. The renewal that opens the
// period charges them, as every change of seats waits for it; the period in hand
// keeps the seats it was paid for. The team's floor and ceiling count every seat.
// holder is as AssignSeats's.
func ChangeSeats(ctx context.Context, org *organization.Organization, holder, id string, quantity int, lines []LineIn) (*Subscription, error) {
	for range mixin.Swaps {
		sub, err := heldSubscription(ctx, org, holder, id)
		if err != nil {
			return nil, err
		}
		slug := subscriptionSlug(sub)
		if !perSeat(slug) {
			return nil, subValidationError{fmt.Sprintf("plan %q has no seats to change", slug)}
		}
		if sub.ProviderType == processorComp {
			return nil, subValidationError{compSeats}
		}
		next, _, err := nextLines(sub.Datastore(), sub, slug, lines)
		if err != nil {
			return nil, err
		}
		own := quantity
		if own <= 0 {
			if own = sub.Quantity; sub.PendingQuantity > 0 {
				own = sub.PendingQuantity
			}
		}
		own = max(own, 1)
		if why := teamSeats(slug, own, next); why != "" {
			return nil, subValidationError{why}
		}
		pendQuantity(sub, quantity)
		pendLines(sub, next)
		ok, err := sub.Swap()
		if err != nil {
			return nil, err
		}
		if ok {
			return viewSubscription(sub), nil
		}
	}
	return nil, fmt.Errorf("subscription %s changed under %d reads in a row", id, mixin.Swaps)
}
