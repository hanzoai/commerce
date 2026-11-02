package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
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
// naming the order, and activates nothing. The same key replays it byte for byte;
// the key with another order is refused; another order for the plan while its
// invoice is open names that invoice.
func TestAnOrderByInvoiceActivatesNothing(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme", "inv-plan")
	out, replay, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-plan", "k1"))
	if err != nil || replay != nil {
		t.Fatalf("order: %v %s", err, replay)
	}
	if out.Status != "open" || out.AmountCents != 20*24000+4000000 || out.BalanceCents != 4000000 || out.Seats != 20 ||
		out.Active || out.SubscriptionID != "" || out.Link == "" || out.Notice != InvoiceNotice {
		t.Fatalf("order: %+v", out)
	}
	if !strings.HasPrefix(out.Reference, "INV-ACME-") || out.Wire == nil || out.Wire.AccountNumber != "000488108457" ||
		out.Wire.Reference != out.Reference || out.Wire.Memo != out.Reference {
		t.Fatalf("pay with: reference %q, wire %+v", out.Reference, out.Wire)
	}
	if d := out.DueAt.Sub(time.Now()); d < 13*24*time.Hour || d > 14*24*time.Hour+time.Minute || !out.VoidAt.Equal(out.DueAt.AddDate(0, 0, 7)) {
		t.Fatalf("due %v, void %v", out.DueAt, out.VoidAt)
	}
	db := datastore.New(org.Namespaced(ctx))
	if subs, _ := userSubscriptions(db, org.Name, org.TestMode()); len(subs) != 0 || balanceOf(t, tc, org, org.Name) != 0 {
		t.Fatalf("an unpaid order opened %d subscriptions and credited %d", len(subs), balanceOf(t, tc, org, org.Name))
	}
	_, again, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-plan", "k1"))
	first, _ := json.Marshal(out)
	if err != nil || !bytes.Equal(again, first) {
		t.Fatalf("the same key should replay the order byte for byte: %v\n%s\n%s", err, again, first)
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
// starts when the payment is confirmed — on the page and in the PDF. A link that
// names nothing is a miss.
func TestTheInvoiceLinkShowsHowToPay(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	ctx := acting(tc)
	org := invoiceSetup(t, ctx, "acme-h", "inv-h-plan")
	out, _, err := OrderInvoice(ctx, org, orderOfInvoice(org, "inv-h-plan", ""))
	if err != nil {
		t.Fatal(err)
	}
	h, err := HostedInvoice(ctx, out.Link)
	if err != nil {
		t.Fatal(err)
	}
	steps := strings.Join(h.HowToPay, "\n")
	for _, want := range []string{"Amount due: $44800.00", "Reference: " + out.Reference, "Include this reference exactly",
		"Bank: Hanzo Bank", "Account number: 000488108457", "Routing number: 026009593", "Account name: Hanzo AI Inc", InvoiceNotice, "voided"} {
		if !strings.Contains(steps, want) {
			t.Fatalf("how to pay lacks %q:\n%s", want, steps)
		}
	}
	if h.BillTo == "" || h.Issuer == "" || h.Wire == nil || h.Number != out.Number || h.Link != "" {
		t.Fatalf("hosted: %+v", h)
	}
	pdf, err := HostedInvoicePDF(ctx, out.Link)
	if err != nil || !bytes.Contains(pdf.Body, []byte(out.Reference)) || !bytes.Contains(pdf.Body, []byte("000488108457")) {
		t.Fatalf("the PDF should carry the reference and the bank: %v", err)
	}
	if _, err := HostedInvoice(ctx, strings.Repeat("x", 43)); !IsSaleNotFound(err) {
		t.Fatalf("an unknown link: %v, want a miss", err)
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
		held != 4000000 || balanceOf(t, tc, org, org.Name) != 4000000 || got.Payment != "wire:FED-2026-0001" {
		t.Fatalf("paid: %+v; host %d, commerce %d", got, held, balanceOf(t, tc, org, org.Name))
	}
	db := datastore.New(org.Namespaced(ctx))
	if held := billingSubscription(db, org.Name, org.TestMode()); held == nil || held.Id() != got.SubscriptionID {
		t.Fatalf("the paid order holds no plan: %+v", held)
	}
	again, err := SettleInvoice(ctx, org, pay)
	if err != nil || again.SubscriptionID != got.SubscriptionID || len(fake.credits()) != 1 {
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
