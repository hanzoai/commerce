// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ap2

import (
	"context"
	"time"
)

// Usage is what has been spent under one open Payment Mandate before: the closed mandates
// already accepted for it. The host keeps it; the constraints payment.agent_recurrence and
// payment.budget are evaluated against it.
type Usage struct {
	// Count is the closed mandates accepted so far.
	Count int64
	// Total is the sum of their amounts in minor units of the budget's currency.
	Total int64
	// Last is when the latest was accepted; zero when none was.
	Last time.Time
}

// Spent reports the Usage of an open Payment Mandate, named by its hash.
type Spent func(ctx context.Context, open string) (Usage, error)

// limits on what an agent-built constraint may ask the verifier to compute.
const (
	maxRequirements = 128
	maxQuantity     = 1 << 31
)

// check evaluates the constraints of an open Checkout Mandate against the checkout
// (checkout_mandate.md, "Constraints"). One that fails, or is not known, is
// unresolved_constraint.
func (o *OpenCheckoutMandate) check(c *Checkout) *Error {
	for _, k := range o.Constraints {
		switch t := k.(type) {
		case AllowedMerchants:
			if c.Merchant == nil {
				return unresolved("the checkout names no merchant")
			}
			if !anyMerchant(t.Allowed, *c.Merchant) {
				return unresolved("the merchant is not among the revealed allowed merchants")
			}
		case LineItems:
			if err := t.fit(c); err != nil {
				return err
			}
		default:
			return unevaluable(k)
		}
	}
	return nil
}

func unevaluable(k Constraint) *Error {
	if u, ok := k.(Unknown); ok {
		return unresolved("constraint %q: %s", u.Type, u.Why)
	}
	return unresolved("constraint %q does not apply to this mandate", k.kind())
}

func anyMerchant(list []Merchant, m Merchant) bool {
	for _, a := range list {
		if sameMerchant(a, m) {
			return true
		}
	}
	return false
}

// fit is the `checkout.line_items` evaluation: the requirements and the checkout's items
// are matched one to one, as a maximum flow from requirements to items. An item matches a
// requirement when its id is among the requirement's revealed acceptable items. The
// constraint holds when the flow equals both the total quantity the requirements ask for and
// the total quantity in the checkout.
func (t LineItems) fit(c *Checkout) *Error {
	if len(t.Items) == 0 || len(t.Items) > maxRequirements {
		return unresolved("checkout.line_items holds 1 to %d items", maxRequirements)
	}
	cart := map[string]int64{}
	var skus []string
	var carted int64
	for _, l := range c.LineItems {
		if l.Quantity > maxQuantity {
			return unresolved("a quantity beyond what is evaluated")
		}
		if _, ok := cart[l.Item.ID]; !ok {
			skus = append(skus, l.Item.ID)
		}
		cart[l.Item.ID] += l.Quantity
		carted += l.Quantity
	}
	if len(skus) == 0 {
		return unresolved("an empty checkout does not satisfy checkout.line_items")
	}
	// nodes: 0 source, 1..R requirements, R+1..R+S items, R+S+1 sink.
	r, s := len(t.Items), len(skus)
	f := newFlow(r + s + 2)
	sink := r + s + 1
	var asked int64
	for i, req := range t.Items {
		if req.Quantity < 1 || req.Quantity > maxQuantity {
			return unresolved("a requirement asks for %d", req.Quantity)
		}
		asked += req.Quantity
		f.add(0, 1+i, req.Quantity)
		for j, sku := range skus {
			for _, ok := range req.AcceptableItems {
				if ok.ID == sku {
					f.add(1+i, 1+r+j, maxQuantity*maxRequirements)
					break
				}
			}
		}
	}
	for j, sku := range skus {
		f.add(1+r+j, sink, cart[sku])
	}
	if got := f.max(0, sink); got != asked || got != carted {
		return unresolved("the checkout's items do not match the line item requirements one to one (%d placed, %d asked, %d in the checkout)", got, asked, carted)
	}
	return nil
}

// flow is a maximum-flow solver (Dinic's algorithm).
type flow struct {
	adj   [][]int
	to    []int
	cap   []int64
	level []int
	next  []int
}

func newFlow(n int) *flow {
	return &flow{adj: make([][]int, n), level: make([]int, n), next: make([]int, n)}
}

func (f *flow) add(u, v int, c int64) {
	f.adj[u] = append(f.adj[u], len(f.to))
	f.to, f.cap = append(f.to, v), append(f.cap, c)
	f.adj[v] = append(f.adj[v], len(f.to))
	f.to, f.cap = append(f.to, u), append(f.cap, 0)
}

func (f *flow) layers(s, t int) bool {
	for i := range f.level {
		f.level[i] = -1
	}
	f.level[s] = 0
	queue := []int{s}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, e := range f.adj[u] {
			if f.cap[e] > 0 && f.level[f.to[e]] < 0 {
				f.level[f.to[e]] = f.level[u] + 1
				queue = append(queue, f.to[e])
			}
		}
	}
	return f.level[t] >= 0
}

func (f *flow) push(u, t int, room int64) int64 {
	if u == t {
		return room
	}
	for ; f.next[u] < len(f.adj[u]); f.next[u]++ {
		e := f.adj[u][f.next[u]]
		v := f.to[e]
		if f.cap[e] <= 0 || f.level[v] != f.level[u]+1 {
			continue
		}
		if got := f.push(v, t, min(room, f.cap[e])); got > 0 {
			f.cap[e] -= got
			f.cap[e^1] += got
			return got
		}
	}
	return 0
}

func (f *flow) max(s, t int) int64 {
	var total int64
	for f.layers(s, t) {
		for i := range f.next {
			f.next[i] = 0
		}
		for {
			got := f.push(s, t, 1<<62)
			if got == 0 {
				break
			}
			total += got
		}
	}
	return total
}

// payEnv is what the payment constraints are evaluated with, beyond the mandates.
type payEnv struct {
	now time.Time
	// openCheckout is the hash of the open Checkout Mandate in the checkout mandate's
	// delegation chain, in the hash algorithm of the open Payment Mandate; "" when the
	// checkout mandate has none.
	openCheckout string
	spent        func() (Usage, *Error)
}

// check evaluates the constraints of an open Payment Mandate against the closed one
// (payment_mandate.md, "Payment Mandate Constraints").
func (o *OpenPaymentMandate) check(p *PaymentMandate, env payEnv) *Error {
	for _, k := range o.Constraints {
		var err *Error
		switch t := k.(type) {
		case AllowedPayees:
			if !anyMerchant(t.Allowed, p.Payee) {
				err = unresolved("the payee is not among the revealed allowed payees")
			}
		case AllowedInstruments:
			err = unresolved("the payment instrument is not among the revealed allowed instruments")
			for _, a := range t.Allowed {
				if a.ID == p.Instrument.ID && a.Type == p.Instrument.Type {
					err = nil
				}
			}
		case AllowedPisps:
			err = unresolved("the PISP is not among the allowed PISPs")
			if p.Pisp != nil {
				for _, a := range t.Allowed {
					if a == *p.Pisp {
						err = nil
					}
				}
			}
		case AmountRange:
			err = t.hold(p.PaymentAmount)
		case ExecutionDate:
			err = t.hold(p.ExecutionDate, env.now)
		case Reference:
			if env.openCheckout == "" || t.ConditionalTransactionID != env.openCheckout {
				err = unresolved("payment.reference does not name the open checkout mandate this payment is presented with")
			}
		case Budget:
			err = t.hold(p.PaymentAmount, env)
		case Recurrence:
			err = t.hold(env)
		default:
			err = unevaluable(k)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (t AmountRange) hold(a Amount) *Error {
	switch {
	case t.Currency == "" || a.Currency != t.Currency:
		return unresolved("the payment is in %s, the amount range in %s", a.Currency, t.Currency)
	case a.Amount > t.Max:
		return unresolved("the payment of %d is above the range's maximum %d", a.Amount, t.Max)
	case t.Min != nil && a.Amount < *t.Min:
		return unresolved("the payment of %d is below the range's minimum %d", a.Amount, *t.Min)
	}
	return nil
}

// hold: the execution date, which is "immediate" when the mandate sets none, lies within
// the window.
func (t ExecutionDate) hold(date string, now time.Time) *Error {
	at := now
	if date != "" {
		var err error
		if at, err = instant(date); err != nil {
			return unresolved("execution_date %q: %v", date, err)
		}
	}
	if t.NotBefore != "" {
		from, err := instant(t.NotBefore)
		if err != nil {
			return unresolved("not_before %q: %v", t.NotBefore, err)
		}
		if at.Before(from) {
			return unresolved("the execution date is before not_before")
		}
	}
	if t.NotAfter != "" {
		to, err := instant(t.NotAfter)
		if err != nil {
			return unresolved("not_after %q: %v", t.NotAfter, err)
		}
		if at.After(to) {
			return unresolved("the execution date is after not_after")
		}
	}
	return nil
}

// instant reads an ISO 8601 date or date-time.
func instant(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse(time.DateOnly, s)
}

// hold: this payment, added to the amounts already accepted under the open mandate, stays
// within the budget. The budget's max is read in the same minor units as the amounts it is
// compared with, and must be a whole number.
func (t Budget) hold(a Amount, env payEnv) *Error {
	max, ok := integer(t.Max)
	if !ok {
		return unresolved("payment.budget max %s is not a whole number of minor units", t.Max)
	}
	if a.Currency != t.Currency {
		return unresolved("the payment is in %s, the budget in %s", a.Currency, t.Currency)
	}
	used, err := env.spent()
	if err != nil {
		return err
	}
	if used.Total+a.Amount > max {
		return unresolved("%d already spent plus %d exceeds the budget of %d", used.Total, a.Amount, max)
	}
	return nil
}

// frequencies are the minimum spacing between two presentations, each as a calendar step.
var frequencies = map[string][3]int{
	"ON_DEMAND": {0, 0, 0},
	"DAILY":     {0, 0, 1},
	"WEEKLY":    {0, 0, 7},
	"BIWEEKLY":  {0, 0, 14},
	"MONTHLY":   {0, 1, 0},
	"QUARTERLY": {0, 3, 0},
	"ANNUALLY":  {1, 0, 0},
}

// hold: this presentation is at least one frequency after the previous one, and within
// max_occurrences.
func (t Recurrence) hold(env payEnv) *Error {
	step, ok := frequencies[t.Frequency]
	if !ok {
		return unresolved("frequency %q", t.Frequency)
	}
	used, err := env.spent()
	if err != nil {
		return err
	}
	if t.MaxOccurrences != nil && used.Count+1 > *t.MaxOccurrences {
		return unresolved("occurrence %d is beyond max_occurrences %d", used.Count+1, *t.MaxOccurrences)
	}
	if !used.Last.IsZero() && env.now.Before(used.Last.AddDate(step[0], step[1], step[2])) {
		return unresolved("the last payment was less than one %s ago", t.Frequency)
	}
	return nil
}
