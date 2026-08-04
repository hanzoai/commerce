// Copyright © 2026 Hanzo AI. MIT License.

package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestPlans_TeamCommercialModel — GET /v1/billing/plans serves the hanzo.team
// commercial model in cents: pro $20, plus $100, max $200, team $25/user with
// per-seat billing, minSeats 2, and the teamGuests limit on paid personal tiers.
func TestPlans_TeamCommercialModel(t *testing.T) {
	a := zip.New(zip.Config{DisableStartupMessage: true})
	a.Get("/v1/billing/plans", ListPlans)

	resp, err := a.Test(httptest.NewRequest(http.MethodGet, "/v1/billing/plans", nil))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET plans: err=%v status=%d", err, resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	var plans []staticPlan
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	bySlug := indexBySlug(plans)

	for slug, cents := range map[string]int64{"go": 900, "dev": 1900, "pro": 4900, "max": 9900, "team": 2500} {
		p, ok := bySlug[slug]
		if !ok {
			t.Fatalf("plan %q missing from GET /v1/billing/plans", slug)
		}
		if p.Price != cents {
			t.Errorf("plan %q price = %d cents, want %d", slug, p.Price, cents)
		}
	}

	team := bySlug["team"]
	if !team.PerSeat {
		t.Error("team must serve perSeat=true (price_ref.recurring.per_seat)")
	}
	if team.Limits == nil || team.Limits.MinSeats == nil || *team.Limits.MinSeats != 2 {
		t.Error("team must serve limits.minSeats=2")
	}
	if team.Limits.IncludedCloudCreditsPerUser == nil || *team.Limits.IncludedCloudCreditsPerUser != 100 {
		t.Error("team must serve limits.includedCloudCreditsPerUser=100")
	}
	pro := bySlug["pro"]
	if pro.Limits == nil || pro.Limits.TeamGuests == nil || *pro.Limits.TeamGuests != 3 {
		t.Error("pro must serve limits.teamGuests=3 (team.guests back-compat source)")
	}
	// The wire carries a real annual discount, not a copy of the monthly price —
	// stated as a relationship so it survives a reprice and still fails the thing
	// that actually went wrong once: annual silently equal to monthly.
	if pro.PriceAnnual <= 0 || pro.PriceAnnual >= pro.Price {
		t.Errorf("pro priceAnnual = %d cents, monthly = %d; annual must be a discount", pro.PriceAnnual, pro.Price)
	}
}

// invokeSubPatch drives UpdateBillingSubscription through real routing.
func invokeSubPatch(org *organization.Organization, ctx context.Context, identity func(*zip.Ctx), id, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPatch, "/v1/billing/subscriptions/"+id, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return driveSeeded(func(c *zip.Ctx) {
		c.Locals("organization", org)
		c.SetContext(ctx)
		identity(c)
	}, "/v1/billing/subscriptions/:id", req, UpdateBillingSubscription)
}

func bodyOf(r *http.Response) string { b, _ := io.ReadAll(r.Body); return string(b) }

// liveMemberSubs returns member's live bundle-child subscriptions.
func liveMemberSubs(t *testing.T, db *datastore.Datastore, member string) []*subscription.Subscription {
	t.Helper()
	subs := make([]*subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", member).GetAll(&subs); err != nil {
		t.Fatalf("query member subs: %v", err)
	}
	out := subs[:0]
	for _, s := range subs {
		if s.ProviderType != "bundle" {
			continue
		}
		switch s.Status {
		case subscription.Active, subscription.Trialing:
			out = append(out, s)
		}
	}
	return out
}

// TestCreateSubscription_TeamBelowMinSeats_Rejected — the catalog's
// limits.minSeats (team: 2) is enforced on create: quantity below the floor
// (including the default 1 when omitted) is a clear 4xx, even for a mint
// principal.
func TestCreateSubscription_TeamBelowMinSeats_Rejected(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("seatsmin")

	for _, body := range []string{
		`{"userId":"seatsmin/owner","planId":"team","quantity":1}`,
		`{"userId":"seatsmin/owner","planId":"team"}`,
	} {
		w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body)
		if w.StatusCode != http.StatusBadRequest {
			t.Fatalf("create team below minSeats: status=%d body=%s, want 400 (%s)", w.StatusCode, bodyOf(w), body)
		}
	}
}

// TestCreateSubscription_TeamSeats_MembersProvisioned — a per-seat team
// subscription records its quantity, and each member gets ONE zero-price
// bundle-child row (the allotment run's per-user grant anchor). Members can
// never exceed seats.
func TestCreateSubscription_TeamSeats_MembersProvisioned(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("seatsteam")
	db := datastore.New(org.Namespaced(ctx))

	// Members above seats is refused outright — member rows mint per-user
	// credit, so they are bounded by the seats actually paid for.
	over := `{"userId":"seatsteam/owner","planId":"team","quantity":2,"members":["seatsteam/a","seatsteam/b","seatsteam/c"]}`
	if w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, over); w.StatusCode != http.StatusBadRequest {
		t.Fatalf("members>seats: status=%d body=%s, want 400", w.StatusCode, bodyOf(w))
	}

	body := `{"userId":"seatsteam/owner","planId":"team","quantity":3,"members":["seatsteam/alice","seatsteam/bob"]}`
	w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription, body)
	if w.StatusCode != 201 {
		t.Fatalf("create team: status=%d body=%s, want 201", w.StatusCode, bodyOf(w))
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(bodyOf(w)), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if q, _ := created["quantity"].(float64); int(q) != 3 {
		t.Fatalf("created quantity = %v, want 3", created["quantity"])
	}

	// Each member holds exactly one live zero-price bundle-child row; the
	// per-user allotment amount for team is $100/mo.
	for _, m := range []string{"seatsteam/alice", "seatsteam/bob"} {
		kids := liveMemberSubs(t, db, m)
		if len(kids) != 1 {
			t.Fatalf("member %s bundle-child rows = %d, want 1", m, len(kids))
		}
		if kids[0].Plan.Price != 0 {
			t.Fatalf("member %s child plan price = %d, want 0 (parent invoice carries the charge)", m, kids[0].Plan.Price)
		}
		if kids[0].Plan.Slug != "team" {
			t.Fatalf("member %s child plan slug = %q, want team", m, kids[0].Plan.Slug)
		}
	}
	if cents := IncludedMonthlyCents("team"); cents != 10000 {
		t.Fatalf("IncludedMonthlyCents(team) = %d, want 10000 ($100/user/mo)", cents)
	}
}

// TestUpdateSubscription_SeatFloorAndMemberIdempotency — PATCH enforces the
// same catalog seat floor, and re-sent members are never duplicated while new
// members gain their row.
func TestUpdateSubscription_SeatFloorAndMemberIdempotency(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	org := moneyOrg("seatspatch")
	db := datastore.New(org.Namespaced(ctx))

	w := invokeSub(org, ctx, c1MintPrincipal, CreateBillingSubscription,
		`{"userId":"seatspatch/owner","planId":"team","quantity":2,"members":["seatspatch/alice"]}`)
	if w.StatusCode != 201 {
		t.Fatalf("create team: status=%d body=%s, want 201", w.StatusCode, bodyOf(w))
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(bodyOf(w)), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	// Below the floor → 400, subscription unchanged.
	if r := invokeSubPatch(org, ctx, c1MintPrincipal, id, `{"quantity":1}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("PATCH quantity=1 on team: status=%d body=%s, want 400", r.StatusCode, bodyOf(r))
	}

	// Members above the effective seats → 400.
	if r := invokeSubPatch(org, ctx, c1MintPrincipal, id,
		`{"quantity":2,"members":["seatspatch/alice","seatspatch/bob","seatspatch/carol"]}`); r.StatusCode != http.StatusBadRequest {
		t.Fatalf("PATCH members>seats: status=%d body=%s, want 400", r.StatusCode, bodyOf(r))
	}

	// Grow to 3 seats, re-sending alice and adding bob: alice keeps exactly
	// one row (idempotent), bob gains one.
	r := invokeSubPatch(org, ctx, c1MintPrincipal, id,
		`{"quantity":3,"members":["seatspatch/alice","seatspatch/bob"]}`)
	if r.StatusCode != 200 {
		t.Fatalf("PATCH grow seats: status=%d body=%s, want 200", r.StatusCode, bodyOf(r))
	}
	for _, m := range []string{"seatspatch/alice", "seatspatch/bob"} {
		if kids := liveMemberSubs(t, db, m); len(kids) != 1 {
			t.Fatalf("member %s bundle-child rows = %d, want exactly 1 (idempotent)", m, len(kids))
		}
	}
}
