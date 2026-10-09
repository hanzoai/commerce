package billing

import (
	"context"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A plan given away. These pin what a comp is: a plan nobody paid for, recorded
// as such, that confers its tier, adds nothing to MRR and never renews.

func compEnterprise(subject string) RecordIn {
	return RecordIn{
		Subject:     subject,
		PlanID:      "enterprise",
		PriceCents:  0,
		PeriodStart: recStart,
		PeriodEnd:   recStart.AddDate(1, 0, 0),
		Processor:   "comp",
		Reference:   map[string]string{"order": "comp-hanzo-2026-10"},
		Terms:       "Hanzo ecosystem org: comp Enterprise, approved by the owner",
	}
}

func TestACompOpensItsPlanAtNoPriceAndNeverRenews(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("comp-open")
	db := datastore.New(org.Namespaced(ctx))

	got, err := RecordSubscription(ctx, org, compEnterprise("hanzo"))
	if err != nil {
		t.Fatalf("record a comp: %v", err)
	}
	if got.Outcome != RecordCreated {
		t.Fatalf("outcome=%q, want %q", got.Outcome, RecordCreated)
	}
	subs := subsOf(t, db, "hanzo")
	if len(subs) != 1 {
		t.Fatalf("subscriptions=%d, want the one comp", len(subs))
	}
	s := subs[0]
	if s.Status != subscription.Active || s.Type != subscription.External || s.ProviderType != "comp" {
		t.Fatalf("status=%q type=%q provider=%q, want an active external row from comp", s.Status, s.Type, s.ProviderType)
	}
	if s.Plan.Price != 0 || SubscriptionMRRCents(s) != 0 || got.Subscription.MRRCents != 0 {
		t.Fatalf("plan price %d, MRR %d / %d: a comp adds nothing to MRR", s.Plan.Price, SubscriptionMRRCents(s), got.Subscription.MRRCents)
	}
	ref, _ := s.Metadata["reference"].(map[string]interface{})
	if ref["order"] != "comp-hanzo-2026-10" || s.Metadata["terms"] == "" {
		t.Fatalf("the approval and terms did not survive the store: %#v", s.Metadata)
	}
	if tier, _ := deriveTier(db, "hanzo", org.TestMode()); tier != "enterprise" {
		t.Fatalf("tier=%q, want the enterprise tier the comp confers", tier)
	}

	if engine.IsDue(s, s.PeriodEnd.Add(24*time.Hour)) {
		t.Fatal("a comp answered due; the cycle would invoice it")
	}
	charged := 0
	burn := &countingPrepaid{}
	charge := func(context.Context, *datastore.Datastore, *billinginvoice.BillingInvoice, int64) (string, error) {
		charged++
		return "ref", nil
	}
	inv, _, err := engine.RenewSubscription(ctx, db, s, burn, charge)
	if err != nil || inv != nil || burn.calls != 0 || charged != 0 {
		t.Fatalf("renewal: invoice=%v burned=%d charged=%d err=%v; a comp is never renewed", inv != nil, burn.calls, charged, err)
	}

	// A retry is unchanged and the next year extends the one row.
	if again, err := RecordSubscription(ctx, org, compEnterprise("hanzo")); err != nil || again.Outcome != RecordUnchanged {
		t.Fatalf("retry: %+v %v, want unchanged", again, err)
	}
	next := compEnterprise("hanzo")
	next.PeriodStart, next.PeriodEnd = next.PeriodEnd, next.PeriodEnd.AddDate(1, 0, 0)
	next.Reference = map[string]string{"order": "comp-hanzo-2027-10"}
	if got, err := RecordSubscription(ctx, org, next); err != nil || got.Outcome != RecordExtended {
		t.Fatalf("next year: %+v %v, want extended", got, err)
	}
}

func TestACompIsRefusedUnlessItIsAComp(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("comp-refuse")
	db := datastore.New(org.Namespaced(ctx))

	for name, mut := range map[string]func(*RecordIn){
		"a price":           func(in *RecordIn) { in.PriceCents = 2000 },
		"no approval":       func(in *RecordIn) { in.Reference = map[string]string{"invoice": "inv-1"} },
		"no terms":          func(in *RecordIn) { in.Terms = " " },
		"another processor": func(in *RecordIn) { in.Processor = "wire" },
	} {
		in := compEnterprise("acme")
		mut(&in)
		if _, err := RecordSubscription(ctx, org, in); !IsSaleRefused(err) {
			t.Errorf("%s: err=%v, want refused", name, err)
		}
	}
	if n := len(subsOf(t, db, "acme")); n != 0 {
		t.Fatalf("refused records wrote %d row(s)", n)
	}

	// A comp and a payment never extend each other; each takes the other over by
	// naming it.
	paid := moneyOrg("comp-paid")
	pdb := datastore.New(paid.Namespaced(ctx))
	dev := recordDev(t, pdb)
	if _, err := RecordSubscription(ctx, paid, dev); err != nil {
		t.Fatalf("record dev: %v", err)
	}
	devComp := compEnterprise("acme")
	devComp.PlanID = "dev"
	if _, err := RecordSubscription(ctx, paid, devComp); !IsSaleConflict(err) {
		t.Fatalf("a comp over a paid plan: err=%v, want conflict", err)
	}
	held := subsOf(t, pdb, "acme")[0]
	devComp.Replaces = held.Id()
	if got, err := RecordSubscription(ctx, paid, devComp); err != nil || got.Outcome != RecordCreated || got.Replaced == nil {
		t.Fatalf("a comp naming the paid plan: %+v %v, want it to take it over", got, err)
	}
	payAgain := recordDev(t, pdb)
	payAgain.PeriodStart, payAgain.PeriodEnd = recEnd, recEnd.AddDate(0, 1, 0)
	if _, err := RecordSubscription(ctx, paid, payAgain); !IsSaleConflict(err) {
		t.Fatalf("a payment over a comp: err=%v, want conflict", err)
	}
}
