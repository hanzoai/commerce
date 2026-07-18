// Copyright © 2026 Hanzo AI. MIT License.

package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/billing/tier"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/test/ae"
)

// tierDeriveDB returns an org-scoped datastore in the SAME namespace the tier
// handlers resolve (org.Namespaced), so a subscription seeded here is the one
// deriveTier / GetTier / TierCheck read back.
func tierDeriveDB(org *organization.Organization) *datastore.Datastore {
	return datastore.New(org.Namespaced(context.Background()))
}

// TestDeriveTier_FromSubscription proves the fix: the tier is derived from the
// subject's REAL subscription instead of the old stub that defaulted everyone to
// Free. active paid → Pro/Enterprise, trialing → Starter, none/$0/canceled → Free.
func TestDeriveTier_FromSubscription(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := moneyOrg("tierderive")
	db := tierDeriveDB(org)

	// active PAID 'pro' → Pro. Seeded as a COMP (manual_gift, no invoice), which
	// also proves tier derivation does NOT apply the money-path payment-backed
	// clamp: a gifted paid subscription still confers its tier.
	seedSub(t, db, "tierderive/pro-user", "pro", subscription.Active, "manual_gift", "")
	// active $200 'max' (personal category) → Pro (all-models tier, not enterprise).
	seedSub(t, db, "tierderive/max-user", "max", subscription.Active, "stripe", "")
	// active 'enterprise' (enterprise category) → Enterprise.
	seedSub(t, db, "tierderive/ent-user", "enterprise", subscription.Active, "internal", "inv_1")
	// trialing entry plan → Starter.
	seedSub(t, db, "tierderive/trial-user", "starter", subscription.Trialing, "trial", "")
	// active $0 'developer' → Free (self-serve, confers no paid tier).
	seedSub(t, db, "tierderive/dev-user", "developer", subscription.Active, "internal", "")
	// canceled 'pro' → Free (only active/trialing confer a tier).
	seedSub(t, db, "tierderive/canceled-user", "pro", subscription.Canceled, "stripe", "")

	cases := []struct {
		user string
		want tier.Name
	}{
		{"tierderive/pro-user", tier.Pro},
		{"tierderive/max-user", tier.Pro},
		{"tierderive/ent-user", tier.Enterprise},
		{"tierderive/trial-user", tier.Starter},
		{"tierderive/dev-user", tier.Free},
		{"tierderive/canceled-user", tier.Free},
		{"tierderive/nobody", tier.Free}, // no subscription at all → genuinely Free
	}
	for _, c := range cases {
		got, err := deriveTier(db, c.user)
		if err != nil {
			t.Fatalf("deriveTier(%q) error: %v", c.user, err)
		}
		if got != c.want {
			t.Errorf("deriveTier(%q) = %q, want %q", c.user, got, c.want)
		}
	}
}

// TestDeriveTier_HighestWins proves a subject holding several subscriptions is
// never under-granted: the highest active/trialing tier wins and inactive
// (canceled) subscriptions are ignored.
func TestDeriveTier_HighestWins(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := moneyOrg("tierhighest")
	db := tierDeriveDB(org)
	user := "tierhighest/multi"

	seedSub(t, db, user, "enterprise", subscription.Canceled, "stripe", "") // ignored
	seedSub(t, db, user, "starter", subscription.Trialing, "trial", "")     // Starter
	seedSub(t, db, user, "pro", subscription.Active, "stripe", "")          // Pro

	got, err := deriveTier(db, user)
	if err != nil {
		t.Fatalf("deriveTier: %v", err)
	}
	if got != tier.Pro {
		t.Fatalf("deriveTier(multi) = %q, want %q (highest active/trialing wins; canceled ignored)", got, tier.Pro)
	}
}

// driveTierJSON drives a tier handler through the real zip stack with the org
// seeded exactly as the gateway/middleware injects it, returning status + parsed body.
func driveTierJSON(org *organization.Organization, routePattern, target string, h zip.Handler) (int, map[string]any) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := driveSeeded(tierSeed(org), routePattern, req, h)
	var out map[string]any
	b, _ := io.ReadAll(w.Body)
	_ = json.Unmarshal(b, &out)
	return w.StatusCode, out
}

func tierNameOf(out map[string]any) string {
	tm, _ := out["tier"].(map[string]any)
	name, _ := tm["name"].(string)
	return name
}

// TestGetTier_ReturnsRealSubscriptionTier drives GET /v1/billing/tier end-to-end
// and proves the wire response carries the subject's REAL tier, not the stubbed Free.
func TestGetTier_ReturnsRealSubscriptionTier(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := moneyOrg("tierwire")
	db := tierDeriveDB(org)
	seedSub(t, db, "tierwire/paige", "pro", subscription.Active, "stripe", "")

	code, out := driveTierJSON(org, "/v1/billing/tier", "/v1/billing/tier?user=tierwire/paige", GetTier)
	if code != 200 {
		t.Fatalf("GetTier status = %d, body=%v", code, out)
	}
	if got := tierNameOf(out); got != string(tier.Pro) {
		t.Fatalf("GetTier tier.name = %q, want %q (real subscription tier, not the stubbed free)", got, tier.Pro)
	}

	code, out = driveTierJSON(org, "/v1/billing/tier", "/v1/billing/tier?user=tierwire/nosub", GetTier)
	if code != 200 {
		t.Fatalf("GetTier(nosub) status = %d", code)
	}
	if got := tierNameOf(out); got != string(tier.Free) {
		t.Fatalf("GetTier(nosub) tier.name = %q, want %q", got, tier.Free)
	}
}

// TestTierCheck_GatesByRealTier drives GET /v1/billing/tier-check and proves the
// model-access gate uses the subject's real tier: a Pro subscriber is allowed any
// model; a Free subject (no subscription) is denied a non-free model.
func TestTierCheck_GatesByRealTier(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	org := moneyOrg("tiercheck")
	db := tierDeriveDB(org)
	seedSub(t, db, "tiercheck/pro", "pro", subscription.Active, "stripe", "")

	// Pro (AllowedModels ["*"]) → any model, including an enso SKU, is allowed.
	code, out := driveTierJSON(org, "/v1/billing/tier-check",
		"/v1/billing/tier-check?user=tiercheck/pro&model=zen4-max", TierCheck)
	if code != 200 {
		t.Fatalf("TierCheck(pro) status = %d", code)
	}
	if got := tierNameOf(out); got != string(tier.Pro) {
		t.Fatalf("TierCheck(pro) tier.name = %q, want %q", got, tier.Pro)
	}
	if allowed, _ := out["allowed"].(bool); !allowed {
		t.Fatalf("TierCheck(pro, zen4-max) allowed=false, want true (Pro allows all models)")
	}

	// Free (no subscription) → a non-free model (zen4-max) is denied.
	code, out = driveTierJSON(org, "/v1/billing/tier-check",
		"/v1/billing/tier-check?user=tiercheck/free&model=zen4-max", TierCheck)
	if code != 200 {
		t.Fatalf("TierCheck(free) status = %d", code)
	}
	if got := tierNameOf(out); got != string(tier.Free) {
		t.Fatalf("TierCheck(free) tier.name = %q, want %q", got, tier.Free)
	}
	if allowed, ok := out["allowed"].(bool); !ok || allowed {
		t.Fatalf("TierCheck(free, zen4-max) allowed=%v, want false (Free excludes zen4)", allowed)
	}
}
