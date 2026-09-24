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
// is a paid tier with max-5x's roster, windows and allotment.
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
	if got, want := coveredCents("max"), coveredCents("max-5x"); got != want || want == 0 {
		t.Fatalf("coveredCents(max) = %d, want max-5x's %d", got, want)
	}
	if got := subscriptionPlanSlug(db, "maxholder/a", false); got != "max" {
		t.Fatalf("the holder's plan reads %q; the subscription keeps the slug it was bought under", got)
	}

	view, err := ReadTier(ctx, org, "maxholder/a", tier.Pro)
	if err != nil {
		t.Fatalf("ReadTier: %v", err)
	}
	if view.Plan != "max-5x" {
		t.Errorf("TierView.Plan = %q, want max-5x — the rung the holder is served as", view.Plan)
	}
	if view.Tier.MaxBots != 1 {
		t.Errorf("max holder may run %d bots, want max-5x's 1", view.Tier.MaxBots)
	}
	want := planWindowLimits("max-5x")
	if len(want) != 4 {
		t.Fatalf("max-5x publishes %d windows, want 4", len(want))
	}
	for _, w := range view.Windows {
		if w.Limit != want[w.Span] {
			t.Errorf("max holder's %s window = %d, want max-5x's %d", w.Span, w.Limit, want[w.Span])
		}
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
