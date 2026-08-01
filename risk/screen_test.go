// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/test/ae"
)

// oracle is the scoring plane a test drives: it records what it was asked and
// answers what the test told it to.
type oracle struct {
	mu     sync.Mutex
	asks   []*Ask
	labels []*Label
	answer *Decision
	err    error
}

func (p *oracle) Decide(_ context.Context, ask *Ask) (*Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.asks = append(p.asks, ask)
	if p.err != nil {
		return nil, p.err
	}
	return p.answer, nil
}

func (p *oracle) Label(_ context.Context, l *Label) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.labels = append(p.labels, l)
	return p.err
}

func (p *oracle) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.asks)
}

// tenant gives a Screener bound to ONE org's namespace — the same binding the
// request path makes, so a test proves the production tenant boundary and not
// a copy of it.
func tenant(name string, ctx context.Context, p Client) *Screener {
	org := &organization.Organization{}
	org.Name = name
	org.Live = true
	return &Screener{DB: datastore.New(org.Namespaced(ctx)), Plane: p, By: "u_test"}
}

func customer(id string) Subject { return Subject{Kind: KindCustomer, ID: id} }

func TestScreen_RecordsTheRefusalWhenNoPlaneIsConfigured(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	// No plane at all: Of() returns the absent one.
	Set(nil)
	s := tenant("noplane", ctx, nil)

	rec, err := s.Screen(context.Background(), Move{Stage: Payment, Subject: customer("c1"), Amount: 500, Currency: currency.USD})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if rec.Refusal != RefusalAbsent {
		t.Fatalf("refusal=%q want %q — a plane that cannot judge must never read as clean", rec.Refusal, RefusalAbsent)
	}
	if rec.Action != string(Allow) {
		t.Fatalf("action=%q want allow — a scoring outage must not stop payments", rec.Action)
	}
	if rec.Id() == "" {
		t.Fatal("the screen was not recorded")
	}
}

func TestScreen_RecordsTheRefusalWhenThePlaneIsUnreachable(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{err: errors.New("dial tcp: i/o timeout")}
	rec, err := tenant("unreach", ctx, p).Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 500, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if rec.Refusal != RefusalUnreachable {
		t.Fatalf("refusal=%q want %q", rec.Refusal, RefusalUnreachable)
	}
	if !Action(rec.Action).Moves() {
		t.Fatalf("action=%q — an unreachable scorer must not stop the payment plane", rec.Action)
	}
}

func TestScreen_EnforcesABlockFromThePlane(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Block, Score: 0.98, Agency: "bot"}}
	rec, err := tenant("blocked", ctx, p).Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 1000, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if !Refused(rec) || rec.Allowed != 0 || rec.Held != 1000 {
		t.Fatalf("blocked screen: action=%s allowed=%d held=%d", rec.Action, rec.Allowed, rec.Held)
	}
	if rec.Decision != "d1" || rec.Agency != "bot" {
		t.Fatalf("the decision was not anchored: %+v", rec)
	}
}

// TestScreen_ShadowIsRecordedAndNotEnforced — a model that quietly went live
// and started blocking payments is the worst failure available here.
func TestScreen_ShadowIsRecordedAndNotEnforced(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Block, Shadow: true}}
	rec, err := tenant("shadow", ctx, p).Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 700, Currency: currency.USD,
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if Refused(rec) {
		t.Fatalf("a shadow judgement must not stop money: action=%s", rec.Action)
	}
	if !rec.Shadow || rec.Decision != "d1" {
		t.Fatalf("the shadow judgement must still be recorded verbatim: %+v", rec)
	}
}

// TestScreen_AControlStillStopsAShadowDecision — a control is the org's own
// standing instruction, not a model's opinion, so shadow mode never lifts it.
func TestScreen_AControlStillStopsAShadowDecision(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("shadowctl", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow, Shadow: true}})
	if _, err := Place(s, customer("c1"), control.Block, 0, time.Time{}, "known mule"); err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{Stage: Payment, Subject: customer("c1"), Amount: 700})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if !Refused(rec) {
		t.Fatalf("a block control must stop the move even in shadow: action=%s", rec.Action)
	}
}

// TestScreen_ACertainBlockCostsNoScoringHop — spending the authorization budget
// on advice that cannot change the outcome is latency an attacker buys.
func TestScreen_ACertainBlockCostsNoScoringHop(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Allow}}
	s := tenant("nohop", ctx, p)
	if _, err := Place(s, customer("c1"), control.Block, 0, time.Time{}, "blocked"); err != nil {
		t.Fatalf("place: %v", err)
	}
	if _, err := s.Screen(context.Background(), Move{Stage: Payment, Subject: customer("c1"), Amount: 100}); err != nil {
		t.Fatalf("screen: %v", err)
	}
	if got := p.count(); got != 0 {
		t.Fatalf("the plane was asked %d times about a move the controls already stop", got)
	}
}

// TestScreen_ReserveHoldsTheExactShareOfAPayout.
func TestScreen_ReserveHoldsTheExactShareOfAPayout(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("reserve", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow}})
	if _, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Reserve, 2500, time.Time{}, "new merchant"); err != nil {
		t.Fatalf("place: %v", err)
	}
	rec, err := s.Screen(context.Background(), Move{
		Stage: Payout, Subject: Subject{Kind: KindMerchant, ID: "m1"}, Amount: 101, Out: true,
	})
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if rec.Held != 26 || rec.Allowed != 75 {
		t.Fatalf("25%% reserve on 101: held=%d allowed=%d want 26/75", rec.Held, rec.Allowed)
	}
	if rec.Held+rec.Allowed != 101 {
		t.Fatalf("the split lost a cent: %d + %d", rec.Held, rec.Allowed)
	}
}

// TestScreen_IsIdempotentOnTheKey — a retried authorization is scored once and
// gets one answer, not two that might differ.
func TestScreen_IsIdempotentOnTheKey(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Allow}}
	s := tenant("idem", ctx, p)
	move := Move{Stage: Payment, Subject: customer("c1"), Amount: 500, Idem: "key-1"}

	first, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := s.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.Id() != second.Id() {
		t.Fatalf("a retry produced a second record: %s vs %s", first.Id(), second.Id())
	}
	if got := p.count(); got != 1 {
		t.Fatalf("the plane was asked %d times for one idempotent move", got)
	}
}

// TestScreen_RefusesASubjectKindThisPlaneCannotName — an org subject would let
// an org's own admin restrain and release a platform restraint on itself.
func TestScreen_RefusesASubjectKindThisPlaneCannotName(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("kinds", ctx, &oracle{answer: &Decision{Action: Allow}})
	for _, bad := range []Subject{{Kind: "org", ID: "acme"}, {Kind: "", ID: "x"}, {Kind: KindCustomer, ID: ""}} {
		if _, err := s.Screen(context.Background(), Move{Stage: Payment, Subject: bad}); err == nil {
			t.Fatalf("subject %+v was accepted", bad)
		}
	}
}

// TestScreen_TenantIsolation is the whole product: one org's screens, controls
// and outcomes are invisible to another, and a foreign subject id gets nothing
// rather than an error that would tell the caller it exists.
func TestScreen_TenantIsolation(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{ID: "d1", Action: Allow}}
	a := tenant("orga", ctx, p)
	b := tenant("orgb", ctx, p)

	rec, err := a.Screen(context.Background(), Move{Stage: Payment, Subject: customer("shared-id"), Amount: 900})
	if err != nil {
		t.Fatalf("a screen: %v", err)
	}
	if _, err := Place(a, customer("shared-id"), control.Block, 0, time.Time{}, "a's fraudster"); err != nil {
		t.Fatalf("a place: %v", err)
	}

	// B names the SAME subject id. It must see none of A's rows.
	if rows := screen.For(b.DB, KindCustomer, "shared-id", 0); len(rows) != 0 {
		t.Fatalf("org B read %d of org A's screens", len(rows))
	}
	if got := control.All(b.DB); len(got) != 0 {
		t.Fatalf("org B read %d of org A's controls", len(got))
	}

	// And B's own move on that id is NOT stopped by A's block.
	brec, err := b.Screen(context.Background(), Move{Stage: Payment, Subject: customer("shared-id"), Amount: 900})
	if err != nil {
		t.Fatalf("b screen: %v", err)
	}
	if Refused(brec) {
		t.Fatalf("org A's control restrained org B's money: %s", brec.Action)
	}

	// B cannot read A's screen by id either.
	foreign := screen.New(b.DB)
	if err := foreign.GetById(rec.Id()); err == nil {
		t.Fatalf("org B read org A's screen %s by id", rec.Id())
	}

	// A still sees its own.
	if rows := screen.For(a.DB, KindCustomer, "shared-id", 0); len(rows) != 1 {
		t.Fatalf("org A sees %d of its own screens, want 1", len(rows))
	}
}

// TestScreen_IdemKeysDoNotCollideAcrossTenants — the same key in two orgs is
// two moves, not one.
func TestScreen_IdemKeysDoNotCollideAcrossTenants(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	p := &oracle{answer: &Decision{Action: Allow}}
	a := tenant("idema", ctx, p)
	b := tenant("idemb", ctx, p)
	move := Move{Stage: Payment, Subject: customer("c1"), Amount: 100, Idem: "same-key"}

	ra, err := a.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	rb, err := b.Screen(context.Background(), move)
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if ra.Id() == rb.Id() {
		t.Fatalf("one idempotency key collapsed two tenants' moves into one record %s", ra.Id())
	}
}

// TestPlace_IsIdempotentWhileTheControlIsInForce — a monitor running every
// cycle must not accumulate a hundred identical holds on one merchant.
func TestPlace_IsIdempotentWhileTheControlIsInForce(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("place", ctx, &oracle{answer: &Decision{Action: Allow}})
	subject := Subject{Kind: KindMerchant, ID: "m1"}
	first, err := Place(s, subject, control.Hold, 0, time.Time{}, "review")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := Place(s, subject, control.Hold, 0, time.Time{}, "review")
		if err != nil {
			t.Fatalf("again: %v", err)
		}
		if again.Id() != first.Id() {
			t.Fatalf("placing twice made a second control: %s vs %s", again.Id(), first.Id())
		}
	}
	if got := control.All(s.DB); len(got) != 1 {
		t.Fatalf("%d controls stored, want 1", len(got))
	}
}

func TestPlace_RefusesAnImpossibleReserveRate(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("rate", ctx, &oracle{answer: &Decision{Action: Allow}})
	for _, rate := range []int64{0, -1, control.FullRate + 1} {
		if _, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Reserve, rate, time.Time{}, ""); err == nil {
			t.Fatalf("reserve rate %d was accepted", rate)
		}
	}
}

// TestPlace_RecordsTheAuthorFromThePrincipal — an author taken off the wire is
// not an audit trail.
func TestPlace_RecordsTheAuthorFromThePrincipal(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("author", ctx, &oracle{answer: &Decision{Action: Allow}})
	c, err := Place(s, Subject{Kind: KindMerchant, ID: "m1"}, control.Hold, 0, time.Time{}, "review")
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if c.By != "u_test" {
		t.Fatalf("by=%q want the screener's principal", c.By)
	}
}
