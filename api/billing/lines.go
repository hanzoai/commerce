// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hanzoai/commerce/datastore"
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
