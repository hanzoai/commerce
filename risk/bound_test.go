// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/outcome"
	"github.com/hanzoai/commerce/models/paymentintent"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// bound_test.go pins that NO READ IN THIS FACE IS UNBOUNDED.
//
// The defect it regresses: a dispute packet read EVERY screen row in the org
// and kept the ones whose reference matched, on a route any authenticated
// caller could hit. One request, one merchant's whole history, one process
// shared by every tenant.

// rows writes n screens into one org.
func rows(t *testing.T, s *Screener, n int, reference string) {
	t.Helper()
	for i := 0; i < n; i++ {
		rec := screen.New(s.DB)
		rec.Stage = string(Payment)
		rec.SubjectKind = KindCustomer
		rec.Subject = "c1"
		rec.Action = string(Allow)
		rec.Reference = reference
		rec.Amount = int64(i + 1)
		rec.Currency = currency.USD
		if err := rec.Create(); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
}

// TestBound_ThereIsNoUnboundedScreenRead — limit 0 is not "everything", a
// caller cannot raise the page by asking for more, and the merchant STANDING
// counted on top of that read inherits the same bound and states it.
func TestBound_ThereIsNoUnboundedScreenRead(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("boundscreens", ctx, &oracle{answer: &Decision{Action: Allow}})
	for i := 0; i < screen.Page+25; i++ {
		rec := screen.New(s.DB)
		rec.Stage, rec.SubjectKind, rec.Subject = string(Payout), KindMerchant, "m1"
		rec.Action, rec.Allowed, rec.Out, rec.Currency = string(Allow), 10, true, currency.USD
		if err := rec.Create(); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	for _, limit := range []int{0, -1, screen.Page, screen.Page + 1, 1_000_000} {
		if got := len(screen.For(s.DB, "", "", limit)); got != screen.Page {
			t.Fatalf("limit=%d returned %d rows, want the page bound %d", limit, got, screen.Page)
		}
	}
	if got := len(screen.For(s.DB, "", "", 7)); got != 7 {
		t.Fatalf("a caller's smaller bound was ignored: %d rows", got)
	}

	// The standing is a ROLLING WINDOW over that same bounded read, and it
	// reports the window so the numbers are interpretable rather than merely
	// bounded.
	st, err := Count(s, merchantOf("m1"))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if st.Screens != screen.Page {
		t.Fatalf("the standing counted %d screens, want the window %d", st.Screens, screen.Page)
	}
	if st.Window != screen.Page {
		t.Fatalf("the standing does not state its window: %d", st.Window)
	}
}

// TestBound_ScreensComeBackNewestFirst — the ordering is what makes the bound
// honest. A bounded read of the OLDEST page is a window on the day the merchant
// signed up, which is worse than no window at all.
func TestBound_ScreensComeBackNewestFirst(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("boundorder", ctx, &oracle{answer: &Decision{Action: Allow}})
	for i := 0; i < 5; i++ {
		rec := screen.New(s.DB)
		rec.Stage, rec.SubjectKind, rec.Subject = string(Payment), KindCustomer, "c1"
		rec.Action, rec.Amount, rec.Currency = string(Allow), int64(i+1), currency.USD
		rec.CreatedAt = time.Date(2026, 8, 1, 0, i, 0, 0, time.UTC)
		if err := rec.Create(); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	got := screen.For(s.DB, "", "", 2)
	if len(got) != 2 {
		t.Fatalf("%d rows, want 2", len(got))
	}
	if got[0].Amount != 5 || got[1].Amount != 4 {
		t.Fatalf("newest-first broken: got amounts %d,%d want 5,4", got[0].Amount, got[1].Amount)
	}
}

// TestBound_ThereIsNoUnboundedOutcomeOrControlRead — the same bound, on the two
// other record planes, for the same reason.
func TestBound_ThereIsNoUnboundedOutcomeOrControlRead(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("boundother", ctx, &oracle{answer: &Decision{Action: Allow}})
	for i := 0; i < outcome.Page+10; i++ {
		o := outcome.New(s.DB)
		o.Event, o.SubjectKind, o.Subject = outcome.Dispute, KindCustomer, "c1"
		if err := o.Create(); err != nil {
			t.Fatalf("seed outcome %d: %v", i, err)
		}
	}
	if got := len(outcome.For(s.DB, "", "", 0)); got != outcome.Page {
		t.Fatalf("outcome.For returned %d, want the page bound %d", got, outcome.Page)
	}

	for i := 0; i < control.Page+10; i++ {
		c := control.New(s.DB)
		c.Effect, c.SubjectKind, c.Subject = control.Block, KindCustomer, string(rune('a'+i%26))+string(rune('a'+i/26))
		if err := c.Create(); err != nil {
			t.Fatalf("seed control %d: %v", i, err)
		}
	}
	if got := len(control.All(s.DB, 0)); got != control.Page {
		t.Fatalf("control.All returned %d, want the page bound %d", got, control.Page)
	}
}

// TestBound_TheDisputePacketDoesNotReadTheWholeOrg is the direct regression.
// The packet is assembled against a store holding far more screens than the
// packet may carry, and it still finds the one that judged the charge — because
// it ASKS for that reference instead of walking everything.
func TestBound_TheDisputePacketDoesNotReadTheWholeOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("boundpacket", ctx, &oracle{answer: &Decision{Action: Allow}})

	// A charge, a dispute on it, and the judgement that admitted it.
	pi := paymentintent.New(s.DB)
	pi.Amount, pi.Currency, pi.CustomerId = 4200, currency.USD, "c1"
	if err := pi.Create(); err != nil {
		t.Fatalf("charge: %v", err)
	}
	admitted := screen.New(s.DB)
	admitted.Stage, admitted.SubjectKind, admitted.Subject = string(Payment), KindCustomer, "c1"
	admitted.Action, admitted.Reference, admitted.Decision = string(Allow), pi.Id(), "d-admitted"
	if err := admitted.Create(); err != nil {
		t.Fatalf("judgement: %v", err)
	}

	// And a great deal of unrelated history, more than any packet may carry.
	rows(t, s, 3*packet, "other")

	d := dispute.New(s.DB)
	d.Amount, d.Currency, d.PaymentIntentId = 4200, currency.USD, pi.Id()
	if err := d.Create(); err != nil {
		t.Fatalf("dispute: %v", err)
	}

	e, err := Assemble(s.DB, d.Id())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(e.Screens) > packet {
		t.Fatalf("the packet carries %d judgements, past its own bound of %d", len(e.Screens), packet)
	}
	found := false
	for _, row := range e.Screens {
		if row.Decision == "d-admitted" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the packet does not cite the judgement that admitted the charge")
	}
	// The truncation is NAMED, because a short packet that looks complete is
	// how a defence gets filed without the evidence it needed.
	named := false
	for _, g := range e.Gaps {
		if len(g) > 10 && g[:10] == "judgements" {
			named = true
		}
	}
	if !named {
		t.Fatalf("the packet was truncated and did not say so: gaps=%v", e.Gaps)
	}
}

// TestBound_AListReportsTheBoundItUsed — a caller must be able to tell a short
// page from a full one without counting.
func TestBound_AListReportsTheBoundItUsed(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("boundreport", ctx, &oracle{answer: &Decision{Action: Allow}})
	if _, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 1, Currency: currency.USD,
	}); err != nil {
		t.Fatalf("screen: %v", err)
	}
	if got := len(screen.ByReference(s.DB, "", 0)); got != 0 {
		t.Fatalf("an empty reference matched %d rows — it must match nothing", got)
	}
}
