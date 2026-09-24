package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
)

// canceledWithAttempt stores a subscription whose renewal charge at d had no
// known outcome, then canceled at once: its renewal invoice is open and carries
// the attempt.
func canceledWithAttempt(t *testing.T, db *datastore.Datastore, d time.Time) (*subscription.Subscription, *billinginvoice.BillingInvoice) {
	t.Helper()
	sub := paidThrough(t, db, monthly, d)
	if step := settleAt(t, db, sub, d, &card{unknown: true}, false); step.Action != Skipped {
		t.Fatalf("unknown outcome = %s, want skipped", step.Action)
	}
	row := reload(t, db, sub.Id())
	End(row, d, d, CanceledAtPeriodEnd)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	inv := storedInvoices(t, db, sub)[0]
	if inv.PendingKey == "" {
		t.Fatal("the renewal invoice carries no attempt")
	}
	return reload(t, db, sub.Id()), inv
}

// looker answers every look-up with looked, or err, and counts them.
func looker(looked Looked, err error, calls *int) Looker {
	return func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice) (Looked, error) {
		*calls++
		return looked, err
	}
}

// TestResolveCanceled_LooksTheCardAttemptUpAndNeverChargesAgain: a canceled
// subscription's card attempt of unknown outcome is settled on what the
// processor says: a payment that landed is recorded and reported for a refund,
// one that never did voids the invoice, and one nobody can place stays on the
// invoice, reported for an operator. No path asks for money, and a dry run
// looks nothing up.
func TestResolveCanceled_LooksTheCardAttemptUpAndNeverChargesAgain(t *testing.T) {
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		looked Looked
		err    error
		dry    bool
		action Action
		status billinginvoice.Status
		kept   bool
		looks  int
	}{
		{"landed", Looked{Known: true, Landed: true, Ref: "sqpay_found"}, nil, false, ChargedAfterCancel, billinginvoice.Paid, false, 1},
		{"never landed", Looked{Known: true}, nil, false, Skipped, billinginvoice.Void, false, 1},
		{"not known", Looked{}, nil, false, Skipped, billinginvoice.Open, true, 1},
		{"look-up failed", Looked{}, errors.New("timeout"), false, Skipped, billinginvoice.Open, true, 1},
		{"dry run", Looked{Known: true, Landed: true}, nil, true, Skipped, billinginvoice.Open, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "resolve-card")
			defer done()
			sub, inv := canceledWithAttempt(t, db, d)
			looks, charged := 0, &card{}
			c := CardPayer(charged.charger())
			c.Look = looker(tc.looked, tc.err, &looks)

			step, err := ResolveCanceled(context.Background(), db, sub, Run{Now: d.Add(time.Hour), DryRun: tc.dry}, c)
			if err != nil {
				t.Fatal(err)
			}
			got, err := loadInvoice(db, inv.Id())
			if err != nil {
				t.Fatal(err)
			}
			if step.Action != tc.action || got.Status != tc.status || (got.PendingKey != "") != tc.kept || looks != tc.looks {
				t.Fatalf("step %s, invoice %s with attempt %q, %d look-ups; want %s, %s, attempt kept %v, %d look-ups",
					step.Action, got.Status, got.PendingKey, looks, tc.action, tc.status, tc.kept, tc.looks)
			}
			if len(charged.amounts) != 0 {
				t.Fatalf("the card was charged %v for a canceled subscription", charged.amounts)
			}
			if tc.action == ChargedAfterCancel && (got.AmountPaid != int64(monthly.Price) || got.PaymentRef != "sqpay_found") {
				t.Fatalf("landed payment recorded as %d cents ref %q, want %d ref sqpay_found", got.AmountPaid, got.PaymentRef, monthly.Price)
			}
		})
	}
}

// TestResolveCanceled_GivesBackAPrepaidAttempt: a canceled subscription's
// prepaid attempt of unknown outcome is repeated under the period's ref, so the
// ledger answers with the draw it holds, and whatever it took goes straight
// back as the invoice is voided.
func TestResolveCanceled_GivesBackAPrepaidAttempt(t *testing.T) {
	db, done := settleDB(t, "resolve-prepaid")
	defer done()
	d := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	inv := issueFor(t, db, sub, d, Advance(d, &monthly))
	inv.PendingMethod, inv.PendingAmount, inv.PendingKey = PaidByPrepaid, inv.AmountDue, AttemptKey(inv)
	if err := inv.Update(); err != nil {
		t.Fatal(err)
	}
	row := reload(t, db, sub.Id())
	End(row, d, d, CanceledAtPeriodEnd)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	p := &purse{balance: 5000}

	step, err := ResolveCanceled(context.Background(), db, reload(t, db, sub.Id()), Run{Now: d, Prepaid: p}, PrepaidPayer(p))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := loadInvoice(db, inv.Id())
	if step.Action != Returned || got.Status != billinginvoice.Void || p.balance != 5000 || len(p.draws) != 1 {
		t.Fatalf("step %s, invoice %s, balance %d after draws %v; want returned, void and the balance whole", step.Action, got.Status, p.balance, p.draws)
	}
}

// TestResolveCanceled_ReturnsARenewalPaidAfterTheLapse: a subscription replaced
// after it lapsed is never served past its paid period, so money its invoice
// collected toward the period after — paid late, before any cycle moved the row
// on — goes back once. A row ended any other way keeps what it was paid.
func TestResolveCanceled_ReturnsARenewalPaidAfterTheLapse(t *testing.T) {
	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		reason Action
		open   bool
		want   Action
	}{
		{"paid, replaced", Replaced, false, Returned},
		{"part paid, replaced", Replaced, true, Returned},
		{"paid, canceled at period end", CanceledAtPeriodEnd, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, done := settleDB(t, "resolve-replaced")
			defer done()
			sub := paidThrough(t, db, monthly, d)
			inv := issueFor(t, db, sub, d, Advance(d, &monthly))
			inv.AmountPaid = inv.AmountDue
			if tc.open {
				inv.AmountPaid = inv.AmountDue / 2
			} else if err := inv.MarkPaid(PaidByCard, "sqpay_late"); err != nil {
				t.Fatal(err)
			}
			if err := inv.Update(); err != nil {
				t.Fatal(err)
			}
			row := reload(t, db, sub.Id())
			End(row, row.PeriodEnd, d.AddDate(0, 3, 0), tc.reason)
			if err := row.Update(); err != nil {
				t.Fatal(err)
			}
			p := &purse{}
			run := Run{Now: d.AddDate(0, 3, 0), Prepaid: p}

			step, err := ResolveCanceled(context.Background(), db, reload(t, db, sub.Id()), run, Collection{})
			if err != nil {
				t.Fatal(err)
			}
			got, _ := loadInvoice(db, inv.Id())
			if step.Action != tc.want {
				t.Fatalf("step %s, want %q", step.Action, tc.want)
			}
			if tc.want == "" {
				if p.balance != 0 || got.Metadata["returnedCents"] != nil {
					t.Fatalf("a row that was not replaced gave back %d cents", p.balance)
				}
				return
			}
			if p.balance != got.AmountPaid || got.Metadata["returnedCents"] == nil || (tc.open && got.Status != billinginvoice.Void) {
				t.Fatalf("balance %d, invoice %s returned %v; want the %d cents collected back", p.balance, got.Status, got.Metadata["returnedCents"], got.AmountPaid)
			}
			again, err := ResolveCanceled(context.Background(), db, reload(t, db, sub.Id()), run, Collection{})
			if err != nil || again.Action != "" || p.balance != got.AmountPaid {
				t.Fatalf("a second run: %s (err %v), balance %d; want nothing more returned", again.Action, err, p.balance)
			}
		})
	}
}

// TestVoidUnpaid_VoidsOnlyWhatCollectedNothing: ending a subscription voids its
// open invoices that collected nothing and carry no attempt, and gives nothing
// back; one that collected money, or whose attempt has no known outcome, is left
// open and named, and a paid one is not touched.
func TestVoidUnpaid_VoidsOnlyWhatCollectedNothing(t *testing.T) {
	db, done := settleDB(t, "void-unpaid")
	defer done()
	d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sub := paidThrough(t, db, monthly, d)
	next := func(i int) time.Time { return d.AddDate(0, i, 0) }
	idle := issueFor(t, db, sub, next(0), next(1))
	part := issueFor(t, db, sub, next(1), next(2))
	part.AmountPaid = part.AmountDue / 2
	pending := issueFor(t, db, sub, next(2), next(3))
	pending.PendingMethod, pending.PendingAmount, pending.PendingKey = PaidByCard, pending.AmountDue, AttemptKey(pending)
	paid := issueFor(t, db, sub, next(3), next(4))
	if err := paid.MarkPaid(PaidByCard, "sqpay"); err != nil {
		t.Fatal(err)
	}
	for _, inv := range []*billinginvoice.BillingInvoice{part, pending, paid} {
		if err := inv.Update(); err != nil {
			t.Fatal(err)
		}
	}

	left, err := VoidUnpaid(db, sub, d)
	if err != nil {
		t.Fatal(err)
	}
	status := func(inv *billinginvoice.BillingInvoice) *billinginvoice.BillingInvoice {
		got, err := loadInvoice(db, inv.Id())
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := status(idle); got.Status != billinginvoice.Void {
		t.Fatalf("the idle invoice is %s, want void", got.Status)
	}
	for _, inv := range []*billinginvoice.BillingInvoice{part, pending} {
		if got := status(inv); got.Status != billinginvoice.Open || got.Metadata["returnedCents"] != nil {
			t.Fatalf("invoice %s is %s (returned %v), want it left open", inv.Id(), got.Status, got.Metadata["returnedCents"])
		}
	}
	if got := status(paid); got.Status != billinginvoice.Paid {
		t.Fatalf("the paid invoice is %s", got.Status)
	}
	if len(left) != 2 {
		t.Fatalf("left %v, want the part-paid and the pending invoice named", left)
	}
}
