package billing

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A record that takes over the plan the subject holds. The new plan is recorded
// first and the held one ends only after, so every refusal leaves the held plan
// active.

const agencyPrice = 20000

// heldMax records the plan the subject holds before the move: max-20x, paid
// outside checkout, on the period running now.
func heldMax(t *testing.T, ctx context.Context, org *organization.Organization, db *datastore.Datastore) *subscription.Subscription {
	t.Helper()
	p, err := resolveSubscriptionPlan(db, "max-20x")
	if err != nil || p.Price <= 0 {
		t.Fatalf("the catalog must sell a paid max-20x plan: %v", err)
	}
	start := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -8)
	got, err := RecordSubscription(ctx, org, RecordIn{
		Subject:     balSubject,
		PlanID:      "max-20x",
		PriceCents:  int64(p.Price),
		PeriodStart: start,
		PeriodEnd:   start.AddDate(0, 1, 0),
		Processor:   "square",
		Reference:   map[string]string{"invoice": "inv:0-max", "payment": "pay_max"},
	})
	if err != nil {
		t.Fatalf("record the held plan: %v", err)
	}
	return reload(t, db, got.Subscription.ID)
}

// agencyIn moves the subject onto the private agency plan, paid by wire, taking
// over replaces.
func agencyIn(replaces string, paid int64) RecordIn {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	return RecordIn{
		Subject:     balSubject,
		PlanID:      "agency",
		PriceCents:  paid,
		PeriodStart: start,
		PeriodEnd:   start.AddDate(0, 1, 0),
		Processor:   "wire",
		Reference:   map[string]string{"payment": "FED-agency"},
		Replaces:    replaces,
		Terms:       "agency, paid by wire",
	}
}

func agencySetup(t *testing.T, ctx context.Context, name string, funded int64) (*organization.Organization, *datastore.Datastore) {
	t.Helper()
	org, db := balanceSetup(t, ctx, name, funded)
	authorityPlan(t, ctx, "agency", "agency", plan.StatusPrivate, agencyPrice)
	return org, db
}

// stillHeld fails unless the row is active, not canceled, and not set to end.
func stillHeld(t *testing.T, db *datastore.Datastore, id string) {
	t.Helper()
	s := reload(t, db, id)
	if s.Status != subscription.Active || s.Canceled || s.EndCancel || !s.CanceledAt.IsZero() || !s.Ended.IsZero() {
		t.Fatalf("the held plan changed: status=%s canceled=%v endCancel=%v canceledAt=%v ended=%v",
			s.Status, s.Canceled, s.EndCancel, s.CanceledAt, s.Ended)
	}
}

// A payment stated one cent short of the price is refused, and the plan the
// subject held stays exactly as it was.
func TestRecordReplaces_ARefusedPaymentLeavesTheHeldPlanActive(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-short", 0)
	held := heldMax(t, ctx, org, db)

	if _, err := RecordSubscription(ctx, org, agencyIn(held.Id(), agencyPrice-1)); !IsSaleRefused(err) {
		t.Fatalf("err = %v, want refused for a price that is not the plan's", err)
	}
	stillHeld(t, db, held.Id())
	if n := len(subsOf(t, db, balSubject)); n != 1 {
		t.Fatalf("a refused record left %d subscription(s), want only the held one", n)
	}
}

// The new plan is opened, and only then does the held one end. A retry answers
// the same plan unchanged.
func TestRecordReplaces_TheHeldPlanEndsOnceTheNewOneIsRecorded(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-paid", 0)
	held := heldMax(t, ctx, org, db)

	got, err := RecordSubscription(ctx, org, agencyIn(held.Id(), agencyPrice))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Outcome != RecordCreated || got.Subscription.Status != string(subscription.Active) {
		t.Fatalf("recorded %+v, want a created active row", got)
	}
	if got.Replaced == nil || got.Replaced.ID != held.Id() || got.Replaced.Status != string(subscription.Canceled) {
		t.Fatalf("replaced = %+v, want %s canceled", got.Replaced, held.Id())
	}
	old := reload(t, db, held.Id())
	if old.Status != subscription.Canceled || old.Ended.IsZero() {
		t.Fatalf("the replaced plan: status=%s ended=%v, want ended at once", old.Status, old.Ended)
	}
	if opened := reload(t, db, got.Subscription.ID).CreatedAt; old.Ended.Before(opened) {
		t.Fatalf("the replaced plan ended at %v, before the new one was opened at %v", old.Ended, opened)
	}
	if hold := billingSubscription(db, balSubject, org.TestMode()); hold == nil || hold.Id() != got.Subscription.ID {
		t.Fatalf("the subject holds %v, want the new plan", hold)
	}

	again, err := RecordSubscription(ctx, org, agencyIn("", agencyPrice))
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if again.Outcome != RecordUnchanged || again.Subscription.ID != got.Subscription.ID {
		t.Fatalf("retry = %+v, want the same plan unchanged", again)
	}
}

// A comp is the plan its subject holds even when its plan is one only the plan
// authority knows (a private plan, Ultra): a record naming it in replaces takes it
// over. A comp is held at price 0, and read from that the account held nothing,
// so replaces was refused as naming no plan. Mutation: drop the comp case in
// paidRow and this record is refused "not a plan this account holds".
func TestRecordReplaces_ACompOfAPrivatePlanIsTheHeldPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-comp", 0)
	authorityPlan(t, ctx, "ultra", "personal", plan.StatusPrivate, 19500)
	start := time.Now().UTC().Truncate(24 * time.Hour)
	comp, err := RecordSubscription(ctx, org, RecordIn{
		Subject: balSubject, PlanID: "ultra", PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0),
		Processor: "comp", Reference: map[string]string{"order": "comp-ultra"}, Terms: "given",
	})
	if err != nil {
		t.Fatalf("comp: %v", err)
	}
	if hold := billingSubscription(db, balSubject, org.TestMode()); hold == nil || hold.Id() != comp.Subscription.ID {
		t.Fatalf("held = %v, want the comp", hold)
	}
	got, err := RecordSubscription(ctx, org, agencyIn(comp.Subscription.ID, agencyPrice))
	if err != nil {
		t.Fatalf("replace the comp: %v", err)
	}
	if got.Replaced == nil || got.Replaced.ID != comp.Subscription.ID || got.Replaced.Status != string(subscription.Canceled) {
		t.Fatalf("replaced = %+v, want the comp ended", got.Replaced)
	}
}

// Replaces must name the plan the subject holds; anything else ends nothing.
func TestRecordReplaces_NamesOnlyTheHeldPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-name", 0)
	held := heldMax(t, ctx, org, db)

	if _, err := RecordSubscription(ctx, org, agencyIn("", agencyPrice)); !IsSaleConflict(err) {
		t.Fatalf("no replaces beside a held plan: err=%v, want conflict", err)
	}
	if _, err := RecordSubscription(ctx, org, agencyIn("not-the-held-plan", agencyPrice)); !IsSaleConflict(err) {
		t.Fatalf("a stale replaces: err=%v, want conflict", err)
	}
	stillHeld(t, db, held.Id())

	bare, bdb := agencySetup(t, ctx, "rep-none", 0)
	if _, err := RecordSubscription(ctx, bare, agencyIn("sub-that-is-not-held", agencyPrice)); !IsSaleRefused(err) {
		t.Fatalf("replaces with no held plan: err=%v, want refused", err)
	}
	if n := len(subsOf(t, bdb, balSubject)); n != 0 {
		t.Fatalf("a refused record wrote %d subscription(s)", n)
	}
}

// Paid outside checkout, a record of another plan takes over the held one the
// same way: recorded first, the held one ended after, and a refused price leaves
// it active.
func TestRecordReplaces_AnExternalRecordTakesOverTheHeldPlan(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-ext", 0)
	held := heldMax(t, ctx, org, db)

	in := agencyIn(held.Id(), agencyPrice-1)
	in.Processor, in.Reference = "wire", map[string]string{"payment": "wire_1"}
	if _, err := RecordSubscription(ctx, org, in); !IsSaleRefused(err) {
		t.Fatalf("err = %v, want refused for a price that is not the plan's", err)
	}
	stillHeld(t, db, held.Id())

	in.PriceCents = agencyPrice
	got, err := RecordSubscription(ctx, org, in)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if got.Outcome != RecordCreated || got.Replaced == nil || got.Replaced.Status != string(subscription.Canceled) {
		t.Fatalf("recorded %+v replaced %+v, want created and the held plan ended", got, got.Replaced)
	}
	if s := reload(t, db, got.Subscription.ID); s.Type != subscription.External || s.Status != subscription.Active {
		t.Fatalf("new row type=%s status=%s, want active external", s.Type, s.Status)
	}

	// The same payment again finds the new plan and changes nothing.
	again, err := RecordSubscription(ctx, org, in)
	if err != nil || again.Outcome != RecordUnchanged || again.Subscription.ID != got.Subscription.ID {
		t.Fatalf("retry = %+v err %v, want the new plan unchanged", again, err)
	}
}

// withdraw is the external path's compensation: the row it opened is gone.
func TestWithdraw_RemovesTheRowItOpened(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org, db := agencySetup(t, ctx, "rep-withdraw", 0)
	held := heldMax(t, ctx, org, db)
	withdraw(db, held)
	if n := len(subsOf(t, db, balSubject)); n != 0 {
		t.Fatalf("withdraw left %d subscription(s)", n)
	}
}
