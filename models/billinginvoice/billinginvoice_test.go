package billinginvoice

import "testing"

// A refund counts once under the id it was given, and refunds never add up to more
// than was paid.
func TestRefundCountsOncePerRef(t *testing.T) {
	inv := &BillingInvoice{AmountPaid: 5000}
	if !inv.Refund("refund:a", 2000) {
		t.Fatal("the first report of a refund was not recorded")
	}
	if inv.Refund("refund:a", 2000) {
		t.Fatal("a refund reported twice was recorded twice")
	}
	if got := inv.Refunded(); got != 2000 {
		t.Fatalf("refunded %d, want 2000", got)
	}
	inv.Refund("refund:b", 4000)
	if got := inv.Refunded(); got != 5000 {
		t.Fatalf("refunded %d, want the 5000 paid", got)
	}
}

// A dispute holds the payment while open or lost; won, it holds nothing, and a later
// report of an earlier state does not reopen it.
func TestADisputeWonStaysWon(t *testing.T) {
	inv := &BillingInvoice{}
	if inv.Disputed() {
		t.Fatal("an invoice nobody disputed reads disputed")
	}
	inv.SetDispute("EVIDENCE_REQUIRED")
	if !inv.Disputed() {
		t.Fatal("an open dispute does not read disputed")
	}
	inv.SetDispute("won")
	if inv.Disputed() {
		t.Fatal("a dispute won reads disputed")
	}
	inv.SetDispute("PROCESSING")
	if inv.Disputed() {
		t.Fatal("a stale report reopened a dispute already won")
	}
	lost := &BillingInvoice{}
	lost.SetDispute("LOST")
	if !lost.Disputed() {
		t.Fatal("a dispute lost does not read disputed")
	}
}
