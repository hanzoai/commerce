package billing

// adjust.go — staff moving the end of the period a subscription holds.
//
// An extension is time given: a card plan renews that much later, and a plan
// collected outside commerce is covered that much longer. A shortening brings the
// renewal, or the lapse, forward. The end is always in the future: ending a plan
// now is a cancel, and moving an end into the past would charge a card at once. A
// card plan scheduled to start at the end that moved starts at the new end instead,
// so the plan it follows and it never overlap or leave a gap. No money moves and
// no invoice is raised; every move is kept on the row (metadata "adjustments").

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	types "github.com/hanzoai/commerce/types"
)

// AdjustIn moves the end of one subscription's period.
type AdjustIn struct {
	// ID is the subscription.
	ID string
	// End is the new end: of the period an active row holds, or the start of a
	// scheduled one. It is in the future.
	End time.Time
	// Note is why, kept on the row.
	Note string
}

// Adjusted is what moving a period did.
type Adjusted struct {
	// Subscription is the row as it now stands; From the end it had.
	Subscription Subscription `json:"subscription"`
	From         time.Time    `json:"from"`
	// Moved is the card plan that was to start at From and now starts at the new
	// end; nil when none waited.
	Moved *Subscription `json:"moved,omitempty"`
}

// AdjustPeriod moves the end of the period subscription in.ID holds — or, for a
// card plan scheduled to start, its start — to in.End. Refused, with nothing
// written, for a row that is not active or scheduled, an end that is not in the
// future or not after the period's start, and while a sale of the subject is in
// flight.
func AdjustPeriod(ctx context.Context, org *organization.Organization, in AdjustIn) (*Adjusted, error) {
	if org == nil {
		return nil, errors.New("adjust period: no organization")
	}
	end, now := in.End.UTC().Truncate(time.Second), time.Now()
	if !end.After(now) {
		return nil, saleRefusal{saleRefused, "the new end is in the future; to end a plan now, cancel it"}
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(strings.TrimSpace(in.ID)); err != nil {
		return nil, saleRefusal{saleMissing, "subscription not found"}
	}
	if strings.EqualFold(strings.TrimSpace(sub.ProviderType), "bundle") {
		return nil, saleRefusal{saleRefused, "a bundled subscription follows the plan it came with; adjust that plan"}
	}
	lease, err := holdSale(db, sub.UserId)
	if err != nil {
		return nil, err
	}
	defer lease.Release()
	// Read again under the lease: a sale or a renewal may have moved it.
	if err := sub.GetById(sub.Id()); err != nil {
		return nil, saleRefusal{saleMissing, "subscription not found"}
	}

	from := sub.PeriodEnd.UTC()
	switch sub.Status {
	case subscription.Active:
		// A row canceled at its end lapses there, so moving the end moves the lapse.
		if !end.After(sub.PeriodStart) {
			return nil, saleRefusal{saleRefused, fmt.Sprintf(
				"subscription %s holds a period from %s; its end is after that", sub.Id(), sub.PeriodStart.UTC().Format(time.RFC3339))}
		}
	case subscription.Scheduled:
	default:
		return nil, saleRefusal{saleRefused, fmt.Sprintf(
			"subscription %s is %s; only an active plan's period, or a scheduled plan's start, is adjusted", sub.Id(), sub.Status)}
	}
	out := &Adjusted{From: from}
	if end.Equal(from) {
		out.Subscription = *viewSubscription(sub)
		return out, nil
	}

	// The card plan that follows this one, when it was to start where this ends.
	var next *subscription.Subscription
	if sub.Status == subscription.Active {
		if w := scheduledFor(db, sub.UserId, sub.Test); w != nil && w.PeriodStart.UTC().Unix() == from.Unix() {
			next = subscription.New(db)
			if err := next.GetById(w.Id()); err != nil {
				return nil, fmt.Errorf("adjust period: the plan scheduled after %s could not be read: %w", sub.Id(), err)
			}
		}
	}

	if sub.Status == subscription.Scheduled {
		sub.Start, sub.PeriodStart = end, end
	}
	sub.PeriodEnd = end
	sub.Metadata = adjustment(sub.Metadata, from, end, in.Note, now)
	if err := sub.Update(); err != nil {
		return nil, fmt.Errorf("adjust period: %w", err)
	}
	out.Subscription = *viewSubscription(sub)
	if next != nil {
		next.Start, next.PeriodStart, next.PeriodEnd = end, end, end
		next.Metadata = adjustment(next.Metadata, from, end, "follows "+sub.Id(), now)
		if err := next.Update(); err != nil {
			return nil, fmt.Errorf("adjust period: subscription %s now ends %s and the plan scheduled to follow it, %s, still starts %s: %w",
				sub.Id(), end.Format(time.RFC3339), next.Id(), from.Format(time.RFC3339), err)
		}
		out.Moved = viewSubscription(next)
	}
	return out, nil
}

// adjustment appends one move of a period's end to a row's metadata.
func adjustment(prev types.Map, from, to time.Time, note string, at time.Time) types.Map {
	m := types.Map{}
	for k, v := range prev {
		m[k] = v
	}
	move := map[string]interface{}{"from": from.Format(time.RFC3339), "to": to.Format(time.RFC3339), "at": at.UTC().Format(time.RFC3339)}
	if n := strings.TrimSpace(note); n != "" {
		move["note"] = n
	}
	moves, _ := m["adjustments"].([]interface{})
	m["adjustments"] = append(moves, move)
	return m
}
