package payment

import (
	"hanzo.io/util/counter"
	"hanzo.io/util/event"
)

// Hooks
func (p *Payment) AfterCreate() error {
	ctx := p.Context()
	now := p.CreatedAt

	amount := 0
	cur := ""

	if p.Type == Stripe && p.CurrencyTransferred != "" {
		amount = int(p.AmountTransferred)
		cur = string(p.CurrencyTransferred)
	} else {
		amount = int(p.Amount)
		cur = string(p.Currency)
	}

	key := counter.Key(p.Kind(), cur)

	// Increment overall sales totals
	counter.IncrementBy(ctx, key, amount)
	counter.IncrementHourBy(ctx, key, now, amount)
	counter.IncrementDayBy(ctx, key, now, amount)
	counter.IncrementMonthBy(ctx, key, now, amount)
	counter.AddSetMember(ctx, "currencies", cur)

	return event.Emit(ctx, p.Namespace(), "payment.created", p)
}

func (p *Payment) AfterUpdate(previous *Payment) error {
	return event.Emit(p.Context(), p.Namespace(), "payment.updated", p)
}

func (p *Payment) AfterDelete() error {
	return event.Emit(p.Context(), p.Namespace(), "payment.deleted", p)
}
