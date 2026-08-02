package billing

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestF1_PatchUpgrade_NonMint_Gated is Red F1: a plan CHANGE had no mint gate, so
// a cheap payment-backed sub could be laundered into a higher tier's spendable
// allotment via a free PATCH (engine.ChangePlan discards its proration). The gate
// refuses a non-mint caller's move that INCREASES the included allotment.
func TestF1_PatchUpgrade_NonMint_Gated(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")
	db := datastore.New(org.Namespaced(ctx))

	// A payment-backed sub at a CHEAP tier (what POST /subscribe/card leaves after a
	// real ~$20 Square charge on "pro": allotment $5).
	s := subscription.New(db)
	s.UserId = "acme/owner"
	s.Status = subscription.Active
	s.ProviderType = "square" // payment-backed
	s.Plan.Slug = "pro"
	s.PlanId = "pro"
	if err := s.Create(); err != nil {
		t.Fatalf("seed sub: %v", err)
	}

	// A NON-mint org owner PATCHes it up to "max" (allotment $100 ≫ pro's $5). Before
	// the fix this succeeded free; now it is 403.
	r := invokeSubPatch(org, ctx, c1OrgAdmin, s.Id(), `{"planId":"max"}`)
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("non-mint PATCH pro→max: status=%d body=%s, want 403 (allotment-increase gated)", r.StatusCode, bodyOf(r))
	}

	// The sub was NOT upgraded — a later allotment/grant anchors on pro ($5), never
	// max ($100). No spendable credit was laundered.
	got := subscription.New(db)
	if err := got.GetById(s.Id()); err != nil {
		t.Fatalf("reload sub: %v", err)
	}
	if got.Plan.Slug != "pro" {
		t.Fatalf("sub plan = %q after gated PATCH, want unchanged 'pro' (no laundering)", got.Plan.Slug)
	}
	// Read both anchors from the catalog rather than restating cents: the property
	// is that the gate left the sub on pro, so the allotment still anchors on pro
	// and never on max. That holds at any price, and a reprice should not fail a
	// test with no opinion about the price.
	wantPro, wantMax := IncludedMonthlyCents("pro"), IncludedMonthlyCents("max")
	if wantPro == wantMax {
		t.Fatal("pro and max mint the same allotment; this test cannot tell laundering from a no-op")
	}
	if got := IncludedMonthlyCents(got.Plan.Slug); got != wantPro {
		t.Fatalf("post-gate allotment anchor = %d, want %d (pro), never %d (max)", got, wantPro, wantMax)
	}

	// Control: a mint principal (cloud-api after a real payment) MAY upgrade — the
	// gate narrows WHO, not the legitimate path.
	s2 := subscription.New(db)
	s2.UserId = "acme/owner2"
	s2.Status = subscription.Active
	s2.ProviderType = "square"
	s2.Plan.Slug = "pro"
	s2.PlanId = "pro"
	if err := s2.Create(); err != nil {
		t.Fatalf("seed sub2: %v", err)
	}
	if r2 := invokeSubPatch(org, ctx, c1MintPrincipal, s2.Id(), `{"planId":"max"}`); r2.StatusCode == http.StatusForbidden {
		t.Fatalf("mint-principal PATCH pro→max: status=403 body=%s, want allowed (authorized mint principal)", bodyOf(r2))
	}
}

// TestF2_DBPriceZero_StillGatedByEmbedPaidTier is Red F2: increment 3a made plan
// Price admin-editable, but the C1-a gate + allotment authority read the embed by
// slug. A SuperAdmin lowering an embed-paid plan's DB Price to 0 (a promo/typo)
// must NOT let an org owner free-provision the paid tier. The gate reads the
// IMMUTABLE embed paidTier, not the DB Price.
func TestF2_DBPriceZero_StillGatedByEmbedPaidTier(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("acme")

	// Seed the authority, then SuperAdmin-edit "max" DB Price → 0.
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	adb := plan.AuthorityDB(ctx)
	p := plan.New(adb)
	if ok, _ := p.Query().Filter("Slug=", "max").Get(); !ok {
		t.Fatal("max not seeded")
	}
	p.Price = 0
	if err := p.Update(); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// Prove the edit LANDED: the charge path now resolves max at Price 0.
	odb := datastore.New(org.Namespaced(ctx))
	rp, err := resolveSubscriptionPlan(odb, "max")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if rp.Price != 0 {
		t.Fatalf("DB edit did not land: resolve(max).Price = %d, want 0", rp.Price)
	}

	// Yet the C1-a gate STILL refuses a non-mint org owner — it reads the embed
	// paidTier("max"), not the DB Price=0. No free provision of the paid tier.
	w := invokeSub(org, ctx, c1OrgAdmin, CreateBillingSubscription, `{"userId":"acme/self","planId":"max"}`)
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("DB Price=0 must NOT bypass C1-a: status=%d body=%s, want 403 (embed paidTier gates)", w.StatusCode, bodyOf(w))
	}
}

// seedAndMaxHashid seeds the plan authority and returns "max"'s DB HASHID — the
// alternate id form resolveSubscriptionPlan→GetById accepts via ByKey, which the
// slug-only gate predicates (IncludedMonthlyCents/paidTier) do NOT match.
func seedAndMaxHashid(t *testing.T, ctx context.Context) string {
	t.Helper()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mx := plan.New(plan.AuthorityDB(ctx))
	if ok, _ := mx.Query().Filter("Slug=", "max").Get(); !ok {
		t.Fatal("max not seeded")
	}
	id := mx.Id()
	if id == "" {
		t.Fatal("max has no hashid")
	}
	return id
}

// TestR1_PatchUpgrade_HashidForm_Gated is Red R1 (F1 half): the F1 allotment gate
// must score the RESOLVED slug, so passing "max"'s DB HASHID gates IDENTICALLY to
// the slug form — a hashid cannot launder a cheap sub into the high allotment.
func TestR1_PatchUpgrade_HashidForm_Gated(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	org := moneyOrg("acme")
	odb := datastore.New(org.Namespaced(c))

	hashid := seedAndMaxHashid(t, c)

	// Sanity: the hashid IS a live resolve path to the high plan, so this genuinely
	// exercises the bypass vector (not a no-op id that would gate trivially).
	rp, err := resolveSubscriptionPlan(odb, hashid)
	if err != nil {
		t.Fatalf("resolve hashid: %v", err)
	}
	if rp.Slug != "max" {
		t.Fatalf("hashid resolved to %q, want max — test not exercising R1", rp.Slug)
	}

	// Payment-backed cheap sub.
	s := subscription.New(odb)
	s.UserId = "acme/owner"
	s.Status = subscription.Active
	s.ProviderType = "square"
	s.Plan.Slug = "pro"
	s.PlanId = "pro"
	if err := s.Create(); err != nil {
		t.Fatalf("seed sub: %v", err)
	}

	// PATCH with the HASHID form — must 403 exactly like the slug form.
	r := invokeSubPatch(org, c, c1OrgAdmin, s.Id(), fmt.Sprintf(`{"planId":%q}`, hashid))
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("non-mint PATCH pro→max(HASHID): status=%d body=%s, want 403 (hashid must not bypass the gate)", r.StatusCode, bodyOf(r))
	}
	got := subscription.New(odb)
	if err := got.GetById(s.Id()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Plan.Slug != "pro" {
		t.Fatalf("sub plan = %q after gated hashid PATCH, want unchanged 'pro' (no laundering)", got.Plan.Slug)
	}
}

// TestR1_Create_HashidForm_Gated is Red R1 (F2 half): the C1-a create gate must
// score the RESOLVED slug, so creating with "max"'s DB HASHID gates identically to
// the slug — a hashid cannot free-provision a paid tier.
func TestR1_Create_HashidForm_Gated(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()
	org := moneyOrg("acme")

	hashid := seedAndMaxHashid(t, c)

	w := invokeSub(org, c, c1OrgAdmin, CreateBillingSubscription, fmt.Sprintf(`{"userId":"acme/self","planId":%q}`, hashid))
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("non-mint create max(HASHID): status=%d body=%s, want 403 (hashid must not bypass C1-a)", w.StatusCode, bodyOf(w))
	}
}
