package billinginvoice

import (
	"testing"
	"time"
)

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

// A dispute holds the payment only while Square withholds its funds — a chargeback,
// or one lost or accepted. Won, it holds nothing, and a later report of an earlier
// state does not reopen it.
func TestADisputeWonStaysWon(t *testing.T) {
	var none time.Time
	inv := &BillingInvoice{}
	if inv.Disputed() {
		t.Fatal("an invoice nobody disputed reads disputed")
	}
	inv.SetDispute("EVIDENCE_REQUIRED", 0, none)
	if !inv.Disputed() {
		t.Fatal("a chargeback does not read disputed")
	}
	inv.SetDispute("won", 0, none)
	if inv.Disputed() {
		t.Fatal("a dispute won reads disputed")
	}
	if inv.SetDispute("PROCESSING", 0, none) || inv.Disputed() {
		t.Fatal("a stale report reopened a dispute already won")
	}
	for _, state := range []string{"LOST", "ACCEPTED", "PROCESSING"} {
		held := &BillingInvoice{}
		held.SetDispute(state, 0, none)
		if !held.Disputed() {
			t.Fatalf("a dispute %s does not read disputed", state)
		}
	}
}

// An inquiry holds no funds, open or closed; a chargeback it escalates to does.
func TestAnInquiryHoldsNothing(t *testing.T) {
	var none time.Time
	inv := &BillingInvoice{}
	for _, state := range []string{"INQUIRY_EVIDENCE_REQUIRED", "INQUIRY_PROCESSING", "INQUIRY_CLOSED"} {
		inv.SetDispute(state, 0, none)
		if inv.Disputed() || inv.Dispute != state {
			t.Fatalf("inquiry %s: disputed %v, recorded %q; want it recorded and holding nothing", state, inv.Disputed(), inv.Dispute)
		}
	}
	inv.SetDispute("EVIDENCE_REQUIRED", 0, none)
	if !inv.Disputed() {
		t.Fatal("an inquiry escalated to a chargeback does not read disputed")
	}
}

// Reports are ordered by the dispute's version, else by its updated_at: an older one
// changes nothing, the same one again is still the latest, and one that cannot be
// ordered is taken.
func TestDisputeReportsAreOrdered(t *testing.T) {
	var none time.Time
	inv := &BillingInvoice{}
	inv.SetDispute("EVIDENCE_REQUIRED", 3, none)
	if inv.SetDispute("INQUIRY_CLOSED", 2, none) || inv.Dispute != "EVIDENCE_REQUIRED" {
		t.Fatalf("an older version was taken: %q", inv.Dispute)
	}
	if !inv.SetDispute("EVIDENCE_REQUIRED", 3, none) {
		t.Fatal("the same report again reads stale")
	}
	if !inv.SetDispute("LOST", 4, none) || inv.Dispute != "LOST" || inv.DisputeVersion != 4 {
		t.Fatalf("a newer version was refused: %q v%d", inv.Dispute, inv.DisputeVersion)
	}

	at := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	byTime := &BillingInvoice{}
	byTime.SetDispute("EVIDENCE_REQUIRED", 0, at)
	if byTime.SetDispute("INQUIRY_CLOSED", 0, at.Add(-time.Hour)) || byTime.Dispute != "EVIDENCE_REQUIRED" {
		t.Fatalf("an older updated_at was taken: %q", byTime.Dispute)
	}
	if !byTime.SetDispute("PROCESSING", 0, none) || byTime.Dispute != "PROCESSING" || !byTime.DisputeAt.Equal(at) {
		t.Fatalf("an unordered report: %q at %s; want it taken and the held time kept", byTime.Dispute, byTime.DisputeAt)
	}

	// Carrying neither, a report is ordered by the dispute's phase: an inquiry
	// cannot follow the chargeback it escalated to.
	byPhase := &BillingInvoice{}
	byPhase.SetDispute("INQUIRY_EVIDENCE_REQUIRED", 0, none)
	byPhase.SetDispute("EVIDENCE_REQUIRED", 0, none)
	if byPhase.SetDispute("INQUIRY_CLOSED", 0, none) || byPhase.Dispute != "EVIDENCE_REQUIRED" {
		t.Fatalf("an inquiry after its chargeback was taken: %q", byPhase.Dispute)
	}
}
