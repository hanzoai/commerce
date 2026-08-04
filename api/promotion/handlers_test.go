package promotion

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/applicationmethod"
	"github.com/hanzoai/commerce/models/campaignbudget"
	promotionModel "github.com/hanzoai/commerce/models/promotion"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// evalOver drives the real Evaluate handler with body over a request whose
// context resolves to org `ns` (Evaluate reads datastore.New(c.Context()), so
// the namespace comes from the context we seed).
func evalOver(t *testing.T, ns string, body evalRequest) evalResponse {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Post("/promotion/evaluate", func(c *zip.Ctx) error {
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		return c.Next()
	}, Evaluate)

	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/promotion/evaluate", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("evaluate status = %d, body=%s", resp.StatusCode, b)
	}
	var out evalResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// seedPromo creates an active automatic promotion + its application method.
func seedPromo(t *testing.T, db *datastore.Datastore, code string, am applicationmethod.ApplicationMethod, window ...*time.Time) *promotionModel.Promotion {
	t.Helper()
	p := promotionModel.New(db)
	p.Code = code
	p.Status = "active"
	p.IsAutomatic = true
	if len(window) == 2 {
		p.StartsAt = window[0]
		p.EndsAt = window[1]
	}
	if err := p.Create(); err != nil {
		t.Fatalf("create promotion: %v", err)
	}

	m := applicationmethod.New(db)
	m.PromotionId = p.Id()
	m.Type = am.Type
	m.TargetType = am.TargetType
	m.Value = am.Value
	m.CurrencyCode = am.CurrencyCode
	if err := m.Create(); err != nil {
		t.Fatalf("create application method: %v", err)
	}
	return p
}

// TestEvaluate_PercentageOrder proves a percentage/order method discounts the
// cart total by basis points (1500 bp = 15%).
func TestEvaluate_PercentageOrder(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	seedPromo(t, db, "SAVE15", applicationmethod.ApplicationMethod{
		Type: "percentage", TargetType: "order", Value: 1500, CurrencyCode: "usd",
	})

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 1500 {
		t.Fatalf("total discount = %d, want 1500 (15%% of 10000)", out.TotalDiscount)
	}
	if len(out.Adjustments) != 1 || out.Adjustments[0].Code != "SAVE15" {
		t.Fatalf("adjustments = %+v, want one for SAVE15", out.Adjustments)
	}
}

// TestEvaluate_Fixed proves a fixed method discounts a flat amount.
func TestEvaluate_Fixed(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	seedPromo(t, db, "FLAT5", applicationmethod.ApplicationMethod{
		Type: "fixed", TargetType: "order", Value: 500, CurrencyCode: "usd",
	})

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 500 {
		t.Fatalf("total discount = %d, want 500", out.TotalDiscount)
	}
}

// TestEvaluate_CurrencyMismatchSkipped proves a method scoped to another currency
// contributes no discount.
func TestEvaluate_CurrencyMismatchSkipped(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	seedPromo(t, db, "EUR15", applicationmethod.ApplicationMethod{
		Type: "percentage", TargetType: "order", Value: 1500, CurrencyCode: "eur",
	})

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 0 || len(out.Adjustments) != 0 {
		t.Fatalf("out = %+v, want no discount (currency mismatch)", out)
	}
}

// TestEvaluate_ExpiredWindowSkipped proves a promotion whose window has ended is
// not applied even though it is active+automatic.
func TestEvaluate_ExpiredWindowSkipped(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	start := time.Now().Add(-48 * time.Hour)
	end := time.Now().Add(-24 * time.Hour) // already ended
	seedPromo(t, db, "EXPIRED", applicationmethod.ApplicationMethod{
		Type: "fixed", TargetType: "order", Value: 500, CurrencyCode: "usd",
	}, &start, &end)

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 0 {
		t.Fatalf("total discount = %d, want 0 (window ended)", out.TotalDiscount)
	}
}

// TestEvaluate_NotYetStartedSkipped proves a promotion whose window has not begun
// is not applied.
func TestEvaluate_NotYetStartedSkipped(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	start := time.Now().Add(24 * time.Hour) // future
	end := time.Now().Add(48 * time.Hour)
	seedPromo(t, db, "FUTURE", applicationmethod.ApplicationMethod{
		Type: "fixed", TargetType: "order", Value: 500, CurrencyCode: "usd",
	}, &start, &end)

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 0 {
		t.Fatalf("total discount = %d, want 0 (not yet started)", out.TotalDiscount)
	}
}

// TestEvaluate_InactiveNotAutomaticExcluded proves the query gate: a draft
// promotion and a non-automatic promotion never enter the calculation.
func TestEvaluate_InactiveNotAutomaticExcluded(t *testing.T) {
	const ns = "acme"
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))

	// draft (not active)
	draft := promotionModel.New(db)
	draft.Code = "DRAFT"
	draft.Status = "draft"
	draft.IsAutomatic = true
	if err := draft.Create(); err != nil {
		t.Fatalf("create draft: %v", err)
	}
	draftAM := applicationmethod.New(db)
	draftAM.PromotionId = draft.Id()
	draftAM.Type = "fixed"
	draftAM.TargetType = "order"
	draftAM.Value = 900
	draftAM.CurrencyCode = "usd"
	if err := draftAM.Create(); err != nil {
		t.Fatalf("create draft am: %v", err)
	}

	// active but code-only (not automatic)
	manual := promotionModel.New(db)
	manual.Code = "MANUAL"
	manual.Status = "active"
	manual.IsAutomatic = false
	if err := manual.Create(); err != nil {
		t.Fatalf("create manual: %v", err)
	}
	manualAM := applicationmethod.New(db)
	manualAM.PromotionId = manual.Id()
	manualAM.Type = "fixed"
	manualAM.TargetType = "order"
	manualAM.Value = 700
	manualAM.CurrencyCode = "usd"
	if err := manualAM.Create(); err != nil {
		t.Fatalf("create manual am: %v", err)
	}

	out := evalOver(t, ns, evalRequest{CurrencyCode: "usd", CartTotal: 10000})
	if out.TotalDiscount != 0 || len(out.Adjustments) != 0 {
		t.Fatalf("out = %+v, want 0 (neither draft nor manual promotion is auto-applied)", out)
	}
}

// TestCampaignBudget_Burndown proves the campaign-budget ledger fields survive
// the datastore round-trip and that spend burns down remaining headroom: Used
// accrues against Limit, and remaining = Limit - Used persists.
func TestCampaignBudget_Burndown(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))

	b := campaignbudget.New(db)
	b.CampaignId = "camp_black_friday"
	b.Type = "spend"
	b.CurrencyCode = "usd"
	b.Limit = 100000 // $1,000 cap
	b.Used = 0
	if err := b.Create(); err != nil {
		t.Fatalf("create budget: %v", err)
	}

	// Burn down: two spends debit the campaign budget.
	got := campaignbudget.New(db)
	if err := got.GetById(b.Id()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got.Used += 30000
	if err := got.Update(); err != nil {
		t.Fatalf("update 1: %v", err)
	}
	got.Used += 45000
	if err := got.Update(); err != nil {
		t.Fatalf("update 2: %v", err)
	}

	final := campaignbudget.New(db)
	if err := final.GetById(b.Id()); err != nil {
		t.Fatalf("reload final: %v", err)
	}
	if final.Used != 75000 {
		t.Fatalf("used = %d, want 75000", final.Used)
	}
	if remaining := final.Limit - final.Used; remaining != 25000 {
		t.Fatalf("remaining = %d, want 25000", remaining)
	}
}
