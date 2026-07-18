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

	"github.com/hanzoai/commerce/api/promo"
	"github.com/hanzoai/commerce/datastore"
	promotionModel "github.com/hanzoai/commerce/models/promotion"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// End-to-end at the API layer: a SuperAdmin PUTs a 50%-off promo on "pro", and
// GET /v1/billing/plans then surfaces promoPercent ONLY on the covered PAID plan —
// not on a free plan, not on an uncovered paid plan. This is the "50% is
// admin-controlled, not hardcoded" proof: the catalog JSON carries no promo.
func TestPlans_SurfaceActivePromo(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	// Warm the reserved platform namespace so the Fiber PUT and the ListPlans read
	// share visibility (ae per-namespace SQLite handle).
	wdb := datastore.New(nscontext.WithNamespace(context.Background(), "admin"))
	_, _ = promotionModel.Query(wdb).Count()

	a := zip.New(zip.Config{DisableStartupMessage: true})
	a.Use(func(c *zip.Ctx) error {
		c.Locals("permissions", bit.Field(permission.Admin|permission.Live)) // SuperAdmin
		return c.Next()
	})
	a.Put("/v1/platform/promo", promo.PutPromo)
	a.Get("/v1/billing/plans", ListPlans)

	// Admin configures a live 50% promo scoped to the "pro" plan.
	putReq := httptest.NewRequest(http.MethodPut, "/v1/platform/promo",
		bytes.NewReader([]byte(`{"percentOff":50,"plans":["pro"],"active":true}`)))
	putReq.Header.Set("Content-Type", "application/json")
	if resp, err := a.Fiber().Test(putReq); err != nil || resp.StatusCode != 200 {
		t.Fatalf("PUT promo: err=%v status=%v", err, statusOf(resp))
	}

	// Read the plan catalog and index by slug.
	getResp, err := a.Fiber().Test(httptest.NewRequest(http.MethodGet, "/v1/billing/plans", nil))
	if err != nil || getResp.StatusCode != 200 {
		t.Fatalf("GET plans: err=%v status=%v", err, statusOf(getResp))
	}
	raw, _ := io.ReadAll(getResp.Body)
	var plans []map[string]any
	if err := json.Unmarshal(raw, &plans); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	bySlug := map[string]map[string]any{}
	for _, p := range plans {
		if s, ok := p["slug"].(string); ok {
			bySlug[s] = p
		}
	}

	// "pro" is a covered PAID plan → promoPercent 50.
	if pct := intField(bySlug["pro"], "promoPercent"); pct != 50 {
		t.Fatalf("pro promoPercent = %d, want 50 (admin promo must surface on the covered paid plan)", pct)
	}
	// "developer" is FREE ($0) → no promo (nothing to discount).
	if pct := intField(bySlug["developer"], "promoPercent"); pct != 0 {
		t.Fatalf("developer promoPercent = %d, want 0 (free plan carries no promo)", pct)
	}
	// "max" is paid but NOT in the promo's plan set → no promo.
	if pct := intField(bySlug["max"], "promoPercent"); pct != 0 {
		t.Fatalf("max promoPercent = %d, want 0 (uncovered plan carries no promo)", pct)
	}
}

func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

func intField(m map[string]any, k string) int {
	if m == nil {
		return 0
	}
	if v, ok := m[k].(float64); ok {
		return int(v)
	}
	return 0
}
