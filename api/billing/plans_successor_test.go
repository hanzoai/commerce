package billing

import (
	"io"
	"net/http"
	"testing"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The $99 max rung left the catalog in @hanzo/plans 1.8.4. Its holders are still
// paying, so every gate that reads the catalog by slug must keep answering for
// them — as max-5x, the rung that replaced it — while the slug itself can no
// longer be bought.

// TestRetiredMaxHolderIsServedAsMax5x: a live, payment-backed max subscription
// is a paid tier with max-5x's roster and allotment.
func TestRetiredMaxHolderIsServedAsMax5x(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	org := moneyOrg("maxholder")
	db := datastore.New(org.Namespaced(ctx))
	seedSub(t, db, "maxholder/a", "max", subscription.Active, "square", "")

	if got, err := deriveTier(db, "maxholder/a", false); err != nil || got != tier.Pro {
		t.Fatalf("deriveTier(max holder) = %q, %v; want Pro — a paying holder must not fall to Free", got, err)
	}
	if !paidTier("max") {
		t.Fatal(`paidTier("max") = false; the holder's renewal still bills, so the tier must read paid`)
	}
	if got, want := listCents("max"), listCents("max-5x"); got != want || want == 0 {
		t.Fatalf("listCents(max) = %d, want max-5x's %d", got, want)
	}
	if got := subscriptionPlanSlug(db, "maxholder/a", false); got != "max" {
		t.Fatalf("the holder's plan reads %q; the subscription keeps the slug it was bought under", got)
	}

	view, err := ReadTier(ctx, org, "maxholder/a")
	if err != nil {
		t.Fatalf("ReadTier: %v", err)
	}
	if view.Plan != "max-5x" {
		t.Errorf("TierView.Plan = %q, want max-5x — the rung the holder is served as", view.Plan)
	}
	if view.Tier.MaxBots != 1 {
		t.Errorf("max holder may run %d bots, want max-5x's 1", view.Tier.MaxBots)
	}
}

// TestRetiredMaxCannotBeBought: the successor answers for holders only. A new
// purchase of the retired slug is refused before the card is touched.
func TestRetiredMaxCannotBeBought(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("maxbuyer")
	m := squareMock("cust_m", "ccof_m", "sqpay_m")
	withFakeSquare(t, m)

	resp := invokeSubscribeCard(org, ctx, `{"sourceId":"cnon:ok","planId":"max"}`, nil)
	if resp.StatusCode != http.StatusNotFound {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s, want 404 — a retired rung is not for sale", resp.StatusCode, string(raw))
	}
	if m.chargeCalls != 0 {
		t.Fatalf("charge calls=%d, want 0", m.chargeCalls)
	}
}

// @hanzo/plans 1.10.0 publishes `go` again, as the $8 entry rung, over a slug a $9
// rung once sold under. Until the boot's migration moves it onto the $8 rung
// (MigrateRetired), a subscription that recorded the old rung renews at the price
// it was bought at, never the catalog's: the renewal bills the snapshot the
// subscription carries.
func TestTheOldGoRenewsAtItsOwnPrice(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	if _, _, err := SeedPlans(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	org := moneyOrg("old-go")
	m := squareMock("", "", "sqpay_oldgo")
	withFakeSquare(t, m)
	db := datastore.New(org.Namespaced(ctx))
	sub := seedCardBackedSub(t, db, "old-go", "go", "ccof_og", "cust_og")
	old := subscription.New(db)
	if err := old.GetById(sub.Id()); err != nil {
		t.Fatalf("load: %v", err)
	}
	old.Plan.Price = 900
	if err := old.Update(); err != nil {
		t.Fatalf("save the $9 snapshot: %v", err)
	}
	if lookupPlan("go").Price != 800 {
		t.Fatalf("the catalog sells go at %d, want 800", lookupPlan("go").Price)
	}
	if r := invokeRenew(org, ctx, sub.Id()); r.StatusCode != http.StatusOK {
		t.Fatalf("renew: %d %s", r.StatusCode, bodyOf(r))
	}
	if m.chargeCalls != 1 || m.lastChargeAmount != 900 {
		t.Fatalf("the old go renewed %d time(s) at %d, want once at its own 900", m.chargeCalls, m.lastChargeAmount)
	}
}
