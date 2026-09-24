package engine

import (
	"errors"
	"fmt"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
)

// Realigned is what Realign did to one subscription, or in a dry run would do:
// the invoice that paid for it, and its period before and after.
type Realigned struct {
	InvoiceId string
	FromStart time.Time
	FromEnd   time.Time
	ToStart   time.Time
	ToEnd     time.Time
}

// Realign moves a subscription that sits a period ahead of the invoice that paid
// for it back onto that invoice's period.
//
// The rule: a row whose PeriodStart is the PeriodEnd of its CurrentInvoiceId
// invoice, where that invoice is the row's own, paid, and bills a real period,
// was moved on a period as the charge was recorded, so its PeriodEnd is one
// period past what was paid for. It is moved onto the invoice's [PeriodStart,
// PeriodEnd], and is due when that period ends. Only a row the cycle renews
// (Live) is considered. Any other row is left alone and nil is answered.
//
// It is an operator's act, run on its own and never as part of reading or
// renewing a subscription. A dry run answers what it would do and writes
// nothing. A real run writes the two dates onto a fresh read of the row, and
// only when that row still matches the rule.
func Realign(db *datastore.Datastore, sub *subscription.Subscription, dryRun bool) (*Realigned, error) {
	inv, err := realignInvoice(db, sub)
	if err != nil || inv == nil {
		return nil, err
	}
	r := &Realigned{
		InvoiceId: inv.Id(),
		FromStart: sub.PeriodStart, FromEnd: sub.PeriodEnd,
		ToStart: inv.PeriodStart, ToEnd: inv.PeriodEnd,
	}
	if dryRun {
		return r, nil
	}
	row := subscription.New(db)
	if err := row.GetById(sub.Id()); err != nil {
		return nil, fmt.Errorf("read subscription %s: %w", sub.Id(), err)
	}
	if again, err := realignInvoice(db, row); err != nil || again == nil || again.Id() != inv.Id() {
		return nil, err
	}
	row.PeriodStart, row.PeriodEnd = inv.PeriodStart, inv.PeriodEnd
	if err := row.Update(); err != nil {
		return nil, fmt.Errorf("realign subscription %s: %w", sub.Id(), err)
	}
	sub.PeriodStart, sub.PeriodEnd = row.PeriodStart, row.PeriodEnd
	return r, nil
}

// realignInvoice is the invoice sub is realigned onto under Realign's rule, or
// nil when the rule does not hold.
func realignInvoice(db *datastore.Datastore, sub *subscription.Subscription) (*billinginvoice.BillingInvoice, error) {
	if !Live(sub) || sub.CurrentInvoiceId == "" || sub.PeriodStart.IsZero() {
		return nil, nil
	}
	inv, err := loadInvoice(db, sub.CurrentInvoiceId)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read invoice %s of subscription %s: %w", sub.CurrentInvoiceId, sub.Id(), err)
	}
	switch {
	case inv.SubscriptionId != sub.Id(),
		inv.Status != billinginvoice.Paid,
		!inv.PeriodStart.Before(inv.PeriodEnd),
		inv.PeriodEnd.Unix() != sub.PeriodStart.Unix():
		return nil, nil
	}
	return inv, nil
}
