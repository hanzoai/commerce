package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/idempotencykey"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/secrets"
	"github.com/hanzoai/commerce/util/test/ae"
)

// invoiceSetup is a buyer org with Hanzo's bank in its secrets and an Enterprise
// row sold by invoice: 14 days to pay, voided 7 days after.
func invoiceSetup(t *testing.T, ctx context.Context, buyer, slug string) *organization.Organization {
	t.Helper()
	webhookOrg(t, ctx, "hanzo", true)
	prev := secrets.Get()
	secrets.Set(brandStore{
		"orgs/hanzo/wire/WIRE_ACCOUNT_NUMBER": "000488108457",
		"orgs/hanzo/wire/WIRE_ROUTING_NUMBER": "026009593",
		"orgs/hanzo/wire/WIRE_BANK_NAME":      "Hanzo Bank",
		"orgs/hanzo/wire/WIRE_ACCOUNT_HOLDER": "Hanzo AI Inc",
	})
	t.Cleanup(func() { secrets.Set(prev) })
	row := orderRow(t, ctx, slug, 20, 2000)
	n := func(v int) *int { return &v }
	row.Limits.InvoiceDays, row.Limits.GraceDays = n(14), n(7)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	return webhookOrg(t, ctx, buyer, true)
}

// acting is ctx with an authenticated caller, as every call the host makes has:
// a brand's bank is read acting for the brand, which needs one.
func acting(ctx context.Context) context.Context {
	return zip.WithCaller(ctx, zip.Caller{Org: "hanzo", User: "ops"})
}

// orderOfInvoice is an order of 20 seats with a $40,000 starting balance.
func orderOfInvoice(org *organization.Organization, slug, key string) OrderIn {
	return OrderIn{
		Subject: org.Name, Email: "ap@" + org.Name + ".test", PlanID: slug, Quantity: 20, Interval: "year",
		BalanceCents: 4000000, Host: "pay.hanzo.ai", IdempotencyKey: key,
	}
}

// An order by invoice raises an open invoice of the seats at the row's price and
// the starting balance, due in the row's days, with Hanzo's bank and a reference
// naming the order, and activates nothing. The same key answers the same invoice,
// its bank and its steps read again and a new link minted, since the guard keeps
// neither; the key with another order is refused; another order for the plan
// while its invoice is open names that invoice.
func TestAnOrderByInvoiceActivatesNothing(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme", "inv-plan")
	out, replay, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-plan", "k1"))
	if err != nil || replay {
		t.Fatalf("order: %v %v", err, replay)
	}
	if out.Status != "open" || out.AmountCents != 20*24000+4000000 || out.BalanceCents != 4000000 || out.Seats != 20 ||
		out.Active || out.SubscriptionID != "" || out.Link == "" || out.Notice != InvoiceNotice {
		t.Fatalf("order: %+v", out)
	}
	if !strings.HasPrefix(out.Reference, "INV-ACME-") || out.Wire == nil || out.Wire.AccountNumber != "000488108457" ||
		out.Wire.Reference != out.Reference || out.Wire.Memo != out.Reference ||
		!strings.Contains(strings.Join(out.HowToPay, "\n"), "Account number: 000488108457") {
		t.Fatalf("pay with: reference %q, wire %+v, steps %q", out.Reference, out.Wire, out.HowToPay)
	}
	if d := out.DueAt.Sub(time.Now()); d < 13*24*time.Hour || d > 14*24*time.Hour+time.Minute || !out.VoidAt.Equal(out.DueAt.AddDate(0, 0, 7)) {
		t.Fatalf("due %v, void %v", out.DueAt, out.VoidAt)
	}
	db := datastore.New(org.Namespaced(ctx))
	if subs, _ := userSubscriptions(db, org.Name, org.TestMode()); len(subs) != 0 || balanceOf(t, tc, org, org.Name) != 0 {
		t.Fatalf("an unpaid order opened %d subscriptions and credited %d", len(subs), balanceOf(t, tc, org, org.Name))
	}
	again, replayed, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-plan", "k1"))
	if err != nil || !replayed || again.InvoiceID != out.InvoiceID || again.Reference != out.Reference ||
		again.Wire == nil || again.Wire.AccountNumber != "000488108457" || strings.Join(again.HowToPay, "\n") != strings.Join(out.HowToPay, "\n") ||
		again.Link == "" || again.Link == out.Link {
		t.Fatalf("the same key should answer the same invoice, read again: %v %v\n%+v", err, replayed, again)
	}
	if h, err := HostedInvoice(ctx, again.Link, "hanzo"); err != nil || h.Order.InvoiceID != out.InvoiceID {
		t.Fatalf("the link a retry minted: %+v %v", h, err)
	}
	other := orderOfInvoice(org, "inv-plan", "k1")
	other.Quantity = 25
	if _, _, err := OrderInvoice(ctx, org, other); !IsSaleConflict(err) {
		t.Fatalf("the key sent with another order: %v, want 409", err)
	}
	if _, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-plan", "k2")); !IsSaleConflict(err) || !strings.Contains(err.Error(), out.InvoiceID) {
		t.Fatalf("a second order while the invoice is open: %v, want 409 naming %s", err, out.InvoiceID)
	}
}

// What a plan or a site does not sell by invoice is refused before anything is
// written: a row stating no invoice days, seats outside the row's figures, a
// balance over its ceiling, and a site with no bank to receive the wire.
func TestAnOrderByInvoiceIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-r", "inv-r-plan")
	orderRow(t, ctx, "card-only-plan", 20, 2000)
	in := orderOfInvoice(org, "card-only-plan", "")
	if _, _, err := OrderInvoice(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("a row with no invoice days: %v", err)
	}
	in = orderOfInvoice(org, "inv-r-plan", "")
	in.Quantity = 19
	if _, _, err := OrderInvoice(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("19 seats under a floor of 20: %v", err)
	}
	in = orderOfInvoice(org, "inv-r-plan", "")
	in.BalanceCents = 5000100
	if _, _, err := OrderInvoice(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("a balance over the ceiling: %v", err)
	}
	secrets.Set(brandStore{})
	if _, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-r-plan", "")); !IsSaleUnavailable(err) {
		t.Fatalf("a site with no bank: %v, want unavailable", err)
	}
	invs := make([]*billinginvoice.BillingInvoice, 0)
	if _, err := billinginvoice.Query(datastore.New(org.Namespaced(ctx))).GetAll(&invs); err != nil || len(invs) != 0 {
		t.Fatalf("refused orders wrote %d invoices (%v)", len(invs), err)
	}
}

// Anyone holding the link reads the invoice and how to pay it — amount, due date,
// the reference to send exactly, the bank one field to a line, and that the plan
// starts when the payment is confirmed — on the page and in the PDF, with the
// order nested under "order" and never the buyer's address. A link that names
// nothing, or a link read on another brand, is a miss.
func TestTheInvoiceLinkShowsHowToPay(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-h", "inv-h-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-h-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	h, err := HostedInvoice(ctx, out.Link, "hanzo")
	if err != nil {
		t.Fatal(err)
	}
	steps := strings.Join(h.Order.HowToPay, "\n")
	for _, want := range []string{"Amount due: $44,800.00", "Reference: " + out.Reference, "Include this reference exactly",
		"Bank: Hanzo Bank", "Account number: 000488108457", "Routing number: 026009593", "Account name: Hanzo AI Inc", InvoiceNotice, "voided"} {
		if !strings.Contains(steps, want) {
			t.Fatalf("how to pay lacks %q:\n%s", want, steps)
		}
	}
	if h.BillTo == "" || h.Issuer == "" || h.Order.Wire == nil || h.Order.Number != out.Number || h.Order.Link != "" || h.Order.Email != "" {
		t.Fatalf("hosted: %+v", h)
	}
	body, _ := json.Marshal(h)
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(body, &shape); err != nil || len(shape["order"]) == 0 || shape["howToPay"] != nil || shape["invoiceId"] != nil ||
		!bytes.Contains(shape["order"], []byte(`"howToPay":[`)) || bytes.Contains(body, []byte("ap@acme-h.test")) {
		t.Fatalf("the hosted answer nests the order, how to pay inside it, and no address:\n%s", body)
	}
	pdf, err := HostedInvoicePDF(ctx, out.Link, "hanzo")
	if err != nil || !bytes.Contains(pdf.Body, []byte(out.Reference)) || !bytes.Contains(pdf.Body, []byte("000488108457")) ||
		bytes.Contains(pdf.Body, []byte("ap@acme-h.test")) {
		t.Fatalf("the PDF should carry the reference and the bank, and not the buyer's address: %v", err)
	}
	if _, err := HostedInvoice(ctx, strings.Repeat("x", 43), "hanzo"); !IsSaleNotFound(err) {
		t.Fatalf("an unknown link: %v, want a miss", err)
	}
	if _, err := HostedInvoice(ctx, out.Link, "lux"); !IsSaleNotFound(err) {
		t.Fatalf("a Hanzo invoice read on Lux: %v, want a miss", err)
	}
	if _, err := HostedInvoicePDF(ctx, out.Link, "lux"); !IsSaleNotFound(err) {
		t.Fatalf("a Hanzo invoice's PDF read on Lux: %v, want a miss", err)
	}
}

// Marked paid by a wire, the order opens the plan and credits the starting
// balance once, in both books, under the payment "wire:<receipt>". The same
// receipt again answers the same; another receipt, or a void, is refused; a
// receipt that settled one invoice settles no other.
func TestAPaidInvoiceOpensThePlanAndCreditsOnce(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-p", "inv-p-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-p-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	pay := SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-2026-0001", AmountCents: out.AmountCents}
	got, err := SettleInvoice(ctx, org, pay)
	if err != nil {
		t.Fatal(err)
	}
	held, _ := fake.Balance(ctx, ledgerOrg(org), org.Name, "usd", org.TestMode())
	if got.SubscriptionID == "" || got.Order.Status != "paid" || got.BalanceCents != 4000000 || got.BalanceID == "" ||
		held != 4000000 || balanceOf(t, tc, org, org.Name) != 4000000 || got.Payment != "wire:FED20260001" || got.Replayed {
		t.Fatalf("paid: %+v; host %d, commerce %d", got, held, balanceOf(t, tc, org, org.Name))
	}
	db := datastore.New(org.Namespaced(ctx))
	if held := billingSubscription(db, org.Name, org.TestMode()); held == nil || held.Id() != got.SubscriptionID {
		t.Fatalf("the paid order holds no plan: %+v", held)
	}
	again, err := SettleInvoice(ctx, org, pay)
	if err != nil || again.SubscriptionID != got.SubscriptionID || len(fake.credits()) != 1 || !again.Replayed {
		t.Fatalf("the same receipt again: %+v %v, credits %d", again, err, len(fake.credits()))
	}
	pay.Receipt = "FED-2026-0002"
	if _, err := SettleInvoice(ctx, org, pay); !IsSaleConflict(err) {
		t.Fatalf("another receipt for a paid invoice: %v", err)
	}
	if _, err := VoidOrder(ctx, org, out.InvoiceID, "staff"); !IsSaleConflict(err) {
		t.Fatalf("voiding a paid invoice: %v", err)
	}
	other := invoiceSetup(t, ctx, "acme-q", "inv-q-plan")
	o2, _, err := OrderInvoice(ctx, other, orderOfInvoice(other, "inv-q-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SettleInvoice(ctx, other, SettleIn{InvoiceID: o2.InvoiceID, Receipt: "FED-2026-0001", AmountCents: o2.AmountCents}); !IsSaleConflict(err) {
		t.Fatalf("a receipt that settled another org's invoice: %v", err)
	}
}

// An amount that is not the invoice's is taken only when the difference is
// stated, and a short payment is short of the starting balance first.
func TestAShortWireIsTakenOnlyWhenStated(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-s", "inv-s-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-s-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	pay := SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-S", AmountCents: out.AmountCents - 1000}
	if _, err := SettleInvoice(ctx, org, pay); !IsSaleRefused(err) {
		t.Fatalf("a short wire with no difference: %v", err)
	}
	pay.Difference = "over"
	if _, err := SettleInvoice(ctx, org, pay); !IsSaleRefused(err) {
		t.Fatalf("a short wire called over: %v", err)
	}
	pay.Difference = "short"
	got, err := SettleInvoice(ctx, org, pay)
	if err != nil || got.ShortCents != 1000 || got.BalanceCents != 4000000-1000 {
		t.Fatalf("a short wire stated: %+v %v", got, err)
	}
}

// Voided, an order activates nothing and cannot be paid; a void and a payment
// that cross are applied one after the other, so exactly one stands. The sweep
// voids an order unpaid past its due date and grace, and only then.
func TestAVoidAndAPaymentNeverBothStand(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-v", "inv-v-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-v-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var voided, paid error
	wg.Add(2)
	go func() { defer wg.Done(); _, voided = VoidOrder(ctx, org, out.InvoiceID, "staff") }()
	go func() {
		defer wg.Done()
		_, paid = SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-V", AmountCents: out.AmountCents})
	}()
	wg.Wait()
	if (voided == nil) == (paid == nil) {
		t.Fatalf("void %v, pay %v: exactly one stands", voided, paid)
	}

	org2 := invoiceSetup(t, ctx, "acme-x", "inv-x-plan")
	o2, _, err := OrderInvoice(ctx, org2, orderOfInvoice(org2, "inv-x-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	if gone, err := ExpireInvoices(ctx, o2.VoidAt.Add(-time.Minute)); err != nil || len(gone) != 0 {
		t.Fatalf("before its grace ends: %+v %v", gone, err)
	}
	gone, err := ExpireInvoices(ctx, o2.VoidAt.Add(time.Minute))
	if err != nil || len(gone) != 1 || gone[0].Org != org2.Name || gone[0].Order.InvoiceID != o2.InvoiceID || gone[0].Order.Status != "void" {
		t.Fatalf("after its grace: %+v %v", gone, err)
	}
	if _, err := SettleInvoice(ctx, org2, SettleIn{InvoiceID: o2.InvoiceID, Receipt: "FED-X", AmountCents: o2.AmountCents}); !IsSaleConflict(err) {
		t.Fatalf("paying a voided invoice: %v", err)
	}
	if subs, _ := userSubscriptions(datastore.New(org2.Namespaced(ctx)), org2.Name, org2.TestMode()); len(subs) != 0 {
		t.Fatalf("a voided order opened %d subscriptions", len(subs))
	}
}

// Under net terms the plan opens with the order and the invoice stays open; paid,
// the same plan is kept and the balance is credited then, not before.
func TestNetTermsOpenThePlanBeforePayment(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-n", "inv-n-plan")
	in := orderOfInvoice(org, "inv-n-plan", "")
	in.Net = true
	out, _, err := OrderInvoice(ctx, org, in)
	if err != nil || !out.Active || out.SubscriptionID == "" || out.Status != "open" || len(fake.credits()) != 0 {
		t.Fatalf("net terms: %+v %v, credits %d", out, err, len(fake.credits()))
	}
	got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-N", AmountCents: out.AmountCents})
	if err != nil || got.SubscriptionID != out.SubscriptionID || got.BalanceCents != 4000000 {
		t.Fatalf("paid under net terms: %+v %v", got, err)
	}
}

// A paid order's wire refunded is taken as a card sale's refund is: the starting
// balance first, then the seats.
func TestARefundOfAWireTakesTheBalanceFirst(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-f", "inv-f-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-f-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-F", AmountCents: out.AmountCents}); err != nil {
		t.Fatal(err)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 1000000, "RF-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 1000000, "RF-1"); err != nil {
		t.Fatal(err)
	}
	held := func() int64 { b, _ := fake.Balance(ctx, ledgerOrg(org), org.Name, "usd", org.TestMode()); return b }
	if held() != 3000000 || balanceOf(t, tc, org, org.Name) != 3000000 {
		t.Fatalf("after a refund of 1,000,000 sent twice: host %d, commerce %d", held(), balanceOf(t, tc, org, org.Name))
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 3480000, "RF-2"); err != nil {
		t.Fatal(err)
	}
	db := datastore.New(org.Namespaced(ctx))
	inv := billinginvoice.New(db)
	if err := inv.GetById(out.InvoiceID); err != nil || held() != 0 || inv.Refunded() != 480000 {
		t.Fatalf("refunded in full: host %d, invoice refunded %d (%v)", held(), inv.Refunded(), err)
	}
}

// settled is org's order of slug paid in full by receipt.
func settled(t *testing.T, ctx context.Context, org *organization.Organization, slug, receipt string) (*InvoiceOrder, *Settled) {
	t.Helper()
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, slug, ""))
	if err != nil {
		t.Fatalf("order %s: %v", slug, err)
	}
	got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: receipt, AmountCents: out.AmountCents})
	if err != nil {
		t.Fatalf("settle %s: %v", slug, err)
	}
	return out, got
}

// live counts the paid plans subject holds.
func live(t *testing.T, db *datastore.Datastore, org *organization.Organization) int {
	t.Helper()
	subs, err := userSubscriptions(db, org.Name, org.TestMode())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range subs {
		if holdsPlan(s) {
			n++
		}
	}
	return n
}

// A wire refunded in full ends the period it paid, as a card refunded in full
// does, and nothing past what was paid is refunded: what a refund took of the
// starting balance counts, though the payment's record holds it and not the
// invoice. (Red, R1.)
func TestAWireRefundedInFullEndsItsPeriod(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "red-r", "red-r-plan")
	out, got := settled(t, ctx, org, "red-r-plan", "FED-R")
	if _, err := RefundOrder(ctx, org, out.InvoiceID, out.AmountCents, "RF-ALL"); err != nil {
		t.Fatal(err)
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(got.SubscriptionID); err != nil {
		t.Fatal(err)
	}
	if pp := periodPaid(db, sub); pp.settled != "" {
		t.Errorf("refunded in full, the plan still holds a paid period: %+v", pp)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 4000000, "RF-MORE"); !IsSaleRefused(err) {
		t.Errorf("a refund past a whole refunded: %v, want refused", err)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, out.AmountCents, "RF-ALL"); err != nil {
		t.Errorf("the same refund again changes nothing and is answered: %v", err)
	}
	if b, _ := fake.Balance(ctx, ledgerOrg(org), org.Name, "usd", org.TestMode()); b != 0 {
		t.Errorf("after a whole refund the wallet holds %d", b)
	}
}

// A refund of the balance alone leaves the seats' period paid, and the rest of
// the wire refundable to the cent.
func TestAWirePartlyRefundedKeepsItsPeriod(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-pr", "inv-pr-plan")
	out, got := settled(t, ctx, org, "inv-pr-plan", "FED-PR")
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 4000000, "RF-B"); err != nil {
		t.Fatal(err)
	}
	db := datastore.New(org.Namespaced(ctx))
	sub := subscription.New(db)
	if err := sub.GetById(got.SubscriptionID); err != nil {
		t.Fatal(err)
	}
	if pp := periodPaid(db, sub); pp.settled != "wire" || pp.charged != 480000 {
		t.Fatalf("the balance refunded, the seats' period reads %+v", pp)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 480001, "RF-S"); !IsSaleRefused(err) {
		t.Fatalf("a cent past what is left: %v, want refused", err)
	}
	if _, err := RefundOrder(ctx, org, out.InvoiceID, 480000, "RF-S"); err != nil {
		t.Fatalf("what is left: %v", err)
	}
}

// One wire settles one invoice, however the bank or a person spells its
// receipt: "fed 2026 0001" is "FED-2026-0001". (Red, R2.)
func TestOneWireSettlesOneInvoiceWhateverItsSpelling(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "red-p", "red-a-plan")
	row := orderRow(t, ctx, "red-b-plan", 20, 2000)
	n := func(v int) *int { return &v }
	row.Limits.InvoiceDays, row.Limits.GraceDays = n(14), n(7)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	settled(t, ctx, org, "red-a-plan", "FED-2026-0001")
	b, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "red-b-plan", ""))
	if err != nil {
		t.Fatalf("second plan order: %v", err)
	}
	for _, spelling := range []string{"fed 2026 0001", "wire:FED20260001", "Fed/2026/0001"} {
		if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: b.InvoiceID, Receipt: spelling, AmountCents: b.AmountCents}); !IsSaleConflict(err) {
			t.Errorf("%q settled a second invoice: %v; credits %d", spelling, err, len(fake.credits()))
		}
	}
	if n := live(t, datastore.New(org.Namespaced(ctx)), org); n != 1 {
		t.Errorf("the org holds %d paid plans", n)
	}
}

// A wire's receipt is one key on both rails: cloud records a wire as credit under
// the same name (apps/admin/core KeyOf), and its tests hold the two equal.
func TestOneWireOneKey(t *testing.T) {
	for in, want := range map[string]string{
		"FED-2026-0001": "wire:FED20260001", "fed 2026 0001": "wire:FED20260001", "wire:FED20260001": "wire:FED20260001",
		"WIRE: wire:fed-2026-0001": "wire:FED20260001", " ft24/1001.xq ": "wire:FT241001XQ", "--": "", "": "", "wire:": "",
	} {
		if got := WireKey(in); got != want {
			t.Errorf("WireKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// A second plan bought by invoice ends the paid plan the org held when it opens,
// as a card sale does, and names it; so does a wire for a plan the org bought by
// card after it ordered. Either way the org holds one paid plan.
func TestASecondPlanByInvoiceEndsTheFirst(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-2", "inv-a2-plan")
	row := orderRow(t, ctx, "inv-b2-plan", 20, 2000)
	n := func(v int) *int { return &v }
	row.Limits.InvoiceDays, row.Limits.GraceDays = n(14), n(7)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	_, a := settled(t, ctx, org, "inv-a2-plan", "FED-A2")
	_, b := settled(t, ctx, org, "inv-b2-plan", "FED-B2")
	db := datastore.New(org.Namespaced(ctx))
	first := subscription.New(db)
	if err := first.GetById(a.SubscriptionID); err != nil || first.Status != subscription.Canceled {
		t.Fatalf("the first plan after the second was paid: %s %v", first.Status, err)
	}
	if b.Order.Replaced != a.SubscriptionID || live(t, db, org) != 1 {
		t.Fatalf("replaced %q (want %s); %d paid plans", b.Order.Replaced, a.SubscriptionID, live(t, db, org))
	}
}

// Ordered by invoice, then bought by card, then paid by wire: the wire's plan
// ends the card's, and the org holds one plan.
func TestACardSaleThenTheWireHoldsOnePlan(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	withFakeSquare(t, squareMock("cust_cw", "ccof_cw", "sqpay_cw"))
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-cw", "inv-cw-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-cw-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	card := subscribed(t, ctx, org, org.Name, "inv-cw-plan", 20, nil)
	got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-CW", AmountCents: out.AmountCents})
	if err != nil {
		t.Fatal(err)
	}
	db := datastore.New(org.Namespaced(ctx))
	was := subscription.New(db)
	if err := was.GetById(card.Id()); err != nil || was.Status != subscription.Canceled || got.Order.Replaced != card.Id() || live(t, db, org) != 1 {
		t.Fatalf("card plan %s (%v), replaced %q, %d paid plans", was.Status, err, got.Order.Replaced, live(t, db, org))
	}
}

// A short payment is short of the starting balance and never of the seats: a cent
// against $44,800.00 opens nothing; the seats alone open the plan with no balance.
// (Red, R3.)
func TestAShortWireCoversTheSeats(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "red-c", "red-c-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "red-c-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, cents := range []int64{1, 479999} {
		if got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-C", AmountCents: cents, Difference: "short"}); !IsSaleRefused(err) {
			t.Fatalf("%d received against %d: %+v %v, want refused", cents, out.AmountCents, got, err)
		}
	}
	got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-C", AmountCents: 480000, Difference: "short"})
	if err != nil || got.SubscriptionID == "" || got.BalanceCents != 0 || got.ShortCents != 4000000 {
		t.Fatalf("the seats alone: %+v %v", got, err)
	}
}

// The retry guard keeps no working link and no bank detail: the link is kept
// only as its digest, the bank only in KMS. (Red, R4.)
func TestTheRetryGuardKeepsNoLinkOrBank(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "red-l", "red-l-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "red-l-plan", "k-red"))
	if err != nil {
		t.Fatal(err)
	}
	db := datastore.New(org.Namespaced(ctx))
	rec, _, err := idempotencykey.Begin(db, invoiceScope+org.Name, "k-red")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec.Response, out.Link) || strings.Contains(rec.Response, "000488108457") || !strings.Contains(rec.Response, out.InvoiceID) {
		t.Errorf("the guard holds %s", rec.Response)
	}
}

// The bank's received date is believed within a day of now and no earlier than
// the invoice's issue date.
func TestAReceivedDateIsBounded(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-d", "inv-d-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-d-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{time.Now().Add(49 * time.Hour), out.IssuedAt.AddDate(0, 0, -2)} {
		if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-D", AmountCents: out.AmountCents, ReceivedAt: at}); !IsSaleRefused(err) {
			t.Fatalf("received %v: %v, want refused", at, err)
		}
	}
	if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-D", AmountCents: out.AmountCents, ReceivedAt: out.IssuedAt}); err != nil {
		t.Fatalf("received the day it was issued: %v", err)
	}
}

// Expiry may void an invoice the bank was already paying: a wire received before
// the void re-opens it and pays it, with the void kept on its record, and a plan
// net terms opened stands again; one received after the void is refused.
func TestAWireReceivedBeforeTheVoidReopensIt(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	for _, net := range []bool{false, true} {
		org := invoiceSetup(t, ctx, fmt.Sprintf("acme-o-%t", net), fmt.Sprintf("inv-o-%t", net))
		in := orderOfInvoice(org, fmt.Sprintf("inv-o-%t", net), "")
		in.Net = net
		out, _, err := OrderInvoice(ctx, org, in)
		if err != nil {
			t.Fatal(err)
		}
		if gone, err := ExpireInvoices(ctx, out.VoidAt.Add(time.Minute)); err != nil || len(gone) == 0 {
			t.Fatalf("expiry: %+v %v", gone, err)
		}
		late := SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-LATE", AmountCents: out.AmountCents, ReceivedAt: time.Now().Add(time.Minute)}
		if _, err := SettleInvoice(ctx, org, late); !IsSaleConflict(err) {
			t.Fatalf("net %t: a wire received after the void: %v, want refused", net, err)
		}
		if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: "FED-NOW", AmountCents: out.AmountCents}); !IsSaleConflict(err) {
			t.Fatalf("net %t: a wire with no received date: %v, want refused", net, err)
		}
		got, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: out.InvoiceID, Receipt: fmt.Sprintf("FED-EARLY-%t", net), AmountCents: out.AmountCents, ReceivedAt: out.IssuedAt})
		if err != nil || !got.Reopened || got.Order.Status != "paid" || !got.Order.Active || (net && got.SubscriptionID != out.SubscriptionID) {
			t.Fatalf("net %t: a wire received before the void: %+v %v", net, got, err)
		}
		inv := billinginvoice.New(datastore.New(org.Namespaced(ctx)))
		o, _ := orderOf(func() *billinginvoice.BillingInvoice { _ = inv.GetById(out.InvoiceID); return inv }())
		if o == nil || o.Reopened == nil || o.Voided == nil || o.Reason != expiry {
			t.Fatalf("net %t: the record keeps the void it undid: %+v", net, o)
		}
	}
}

// An org that let an order under net terms expire unpaid is not started before
// it pays again: its next order, net terms or not, opens nothing until paid.
func TestNetTermsLapseWithAnOrderLeftUnpaid(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-l", "inv-l-plan")
	in := orderOfInvoice(org, "inv-l-plan", "")
	in.Net = true
	first, _, err := OrderInvoice(ctx, org, in)
	if err != nil || !first.Active {
		t.Fatalf("net terms: %+v %v", first, err)
	}
	if _, err := ExpireInvoices(ctx, first.VoidAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	in.IdempotencyKey = "again"
	again, _, err := OrderInvoice(ctx, org, in)
	if err != nil || again.Active || again.Net || again.SubscriptionID != "" {
		t.Fatalf("after an unpaid net order expired: %+v %v", again, err)
	}
}

// References that a bank would read alike once it drops separators differ.
func TestReferencesDifferWithoutSeparators(t *testing.T) {
	squeeze := func(s string) string { return strings.TrimPrefix(WireKey(s), "wire:") }
	for _, p := range [][2]struct {
		org string
		n   int
	}{{{"acme-io", 42}, {"acmeio", 42}}, {{"acme1", 42}, {"acme", 10042}}} {
		a, b := invoiceReference(p[0].org, p[0].n), invoiceReference(p[1].org, p[1].n)
		if squeeze(a) == squeeze(b) {
			t.Errorf("%s and %s read alike: %s", a, b, squeeze(a))
		}
	}
	if r := invoiceReference("acme", 42); !strings.HasPrefix(r, "INV-ACME-0042-") || len(r) != len("INV-ACME-0042-")+8 {
		t.Errorf("reference %q", r)
	}
}

// A row sold by invoice is an organization's order, so it is sold to the
// organization's own account and to no member, whichever way it is paid.
func TestAnInvoiceRowIsAnOrganizationsOrder(t *testing.T) {
	n := 14
	p := &plan.Plan{Limits: &plan.Limits{InvoiceDays: &n}}
	org := moneyOrg("acme-o")
	if code, _ := orderRefusal(org, p, "ent", SubscribeIn{Subject: "acme-o/ada"}, ""); !p.Limits.Ordered() || code != "org_required" {
		t.Fatalf("an invoice row sold to a member")
	}
}

// codeOf is the code a refusal names, "" for none.
func codeOf(err error) string {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return ""
}

// Each refusal a page acts on names its code, so the page reads the code and not
// the sentence; the one for an invoice still open names that invoice with a new
// link to it, the order's own link staying valid beside it.
func TestOrderRefusalsNameTheirCode(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-k", "inv-k-plan")
	orderRow(t, ctx, "card-k-plan", 20, 2000)
	first, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-k-plan", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = OrderInvoice(ctx, org, orderOfInvoice(org, "inv-k-plan", "k2"))
	var open *Refusal
	if !IsSaleConflict(err) || !errors.As(err, &open) || open.Code != "invoice_open" || open.Open == nil ||
		open.Open.InvoiceID != first.InvoiceID || open.Open.Number != first.Number || open.Open.Link == "" || open.Open.Link == first.Link {
		t.Fatalf("an open invoice: %v %+v", err, open)
	}
	for _, link := range []string{first.Link, open.Open.Link} {
		if h, err := HostedInvoice(ctx, link, "hanzo"); err != nil || h.Order.InvoiceID != first.InvoiceID {
			t.Fatalf("link %s: %+v %v", link, h, err)
		}
	}
	other := orderOfInvoice(org, "inv-k-plan", "k1")
	other.Quantity = 25
	cases := map[string]OrderIn{"key_reused": other}
	in := orderOfInvoice(org, "card-k-plan", "")
	cases["not_sold_by_invoice"] = in
	in = orderOfInvoice(org, "inv-k-plan", "")
	in.Quantity = 19
	cases["seats_out_of_range"] = in
	in = orderOfInvoice(org, "inv-k-plan", "")
	in.BalanceCents = 5000100
	cases["balance_out_of_range"] = in
	in = orderOfInvoice(org, "inv-k-plan", "")
	in.Subject = org.Name + "/ada"
	cases["org_required"] = in
	for want, in := range cases {
		if _, _, err := OrderInvoice(ctx, org, in); codeOf(err) != want {
			t.Errorf("%s: %v (code %q)", want, err, codeOf(err))
		}
	}
	db := datastore.New(org.Namespaced(ctx))
	if _, _, err := idempotencykey.Begin(db, invoiceScope+org.Name, "k-flight"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-k-plan", "k-flight")); !IsSaleConflict(err) || codeOf(err) != "order_placing" {
		t.Errorf("an order in flight: %v (code %q)", err, codeOf(err))
	}
	if _, err := SettleInvoice(ctx, org, SettleIn{InvoiceID: first.InvoiceID, Receipt: "FED-K", AmountCents: first.AmountCents}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-k-plan", "k3")); !IsSaleConflict(err) || codeOf(err) != "plan_held" {
		t.Errorf("a plan held: %v (code %q)", err, codeOf(err))
	}
	secrets.Set(brandStore{})
	if _, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-k-plan", "k4")); !IsSaleUnavailable(err) || codeOf(err) != "wire_not_configured" {
		t.Errorf("no bank: %v (code %q)", err, codeOf(err))
	}
}

// Staff read one order invoice whole — how to pay it while open, the payment
// once paid — and an org's order invoices by status, newest first.
func TestStaffReadAnOrgsOrders(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-st", "inv-st-plan")
	row := orderRow(t, ctx, "inv-st2-plan", 20, 2000)
	n := func(v int) *int { return &v }
	row.Limits.InvoiceDays, row.Limits.GraceDays = n(14), n(7)
	if err := row.Update(); err != nil {
		t.Fatal(err)
	}
	paid, _ := settled(t, ctx, org, "inv-st-plan", "FED-ST")
	open, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-st2-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Order(ctx, org, open.InvoiceID)
	if err != nil || got.Wire == nil || got.Wire.AccountNumber != "000488108457" || len(got.HowToPay) == 0 || got.Link != "" || got.Reference != open.Reference {
		t.Fatalf("an open order: %+v %v", got, err)
	}
	if got, err := Order(ctx, org, paid.InvoiceID); err != nil || got.Payment != "wire:FEDST" || got.Wire != nil || !got.Active {
		t.Fatalf("a paid order: %+v %v", got, err)
	}
	if _, err := Order(ctx, org, "nope"); !IsSaleNotFound(err) {
		t.Fatalf("an unknown invoice: %v", err)
	}
	all, err := Orders(ctx, org, "")
	if err != nil || len(all) != 2 || all[0].InvoiceID != open.InvoiceID || all[0].Wire != nil {
		t.Fatalf("every order: %+v %v", all, err)
	}
	if opens, err := Orders(ctx, org, "open"); err != nil || len(opens) != 1 || opens[0].InvoiceID != open.InvoiceID {
		t.Fatalf("open orders: %+v %v", opens, err)
	}
}

// A plan paid by wire is collected outside commerce: no renewal takes its next
// period from the org's balance or a card.
func TestAWirePlanIsNeverCollected(t *testing.T) {
	fake := newFakeLedger()
	injectLedger(t, fake)
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-e", "inv-e-plan")
	_, got := settled(t, ctx, org, "inv-e-plan", "FED-E")
	sub := subscription.New(datastore.New(org.Namespaced(ctx)))
	if err := sub.GetById(got.SubscriptionID); err != nil {
		t.Fatal(err)
	}
	if sub.Type != subscription.External || engine.IsDue(sub, sub.PeriodEnd.AddDate(0, 0, 1)) {
		t.Fatalf("a wire plan is %q and due after its period: %v", sub.Type, engine.IsDue(sub, sub.PeriodEnd.AddDate(0, 0, 1)))
	}
}
