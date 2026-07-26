package draftorder

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/datastore"
	draftorderModel "github.com/hanzoai/commerce/models/draftorder"
	"github.com/hanzoai/commerce/models/draftorderitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// callComplete drives the Complete handler over a real request wired so
// middleware.GetOrganization(c) + org.Namespaced(c.Context()) resolve to the ae
// SQLite datastore in org `ns`'s namespace — the exact production plumbing (org
// name IS the namespace). admin selects whether the injected IAM claim
// authorizes the admin-gated complete action. Returns status and body.
func callComplete(t *testing.T, ns string, admin bool, draftID string) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns, IsAdmin: admin})
		return c.Next()
	}
	app.Post("/draftorder/:draftorderid/complete", seed, Complete)

	req := httptest.NewRequest(http.MethodPost, "/draftorder/"+draftID+"/complete", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// seedDraft creates a draft order and its line items in org `ns`.
func seedDraft(t *testing.T, base context.Context, ns, email string, lines []draftorderitem.DraftOrderItem) *draftorderModel.DraftOrder {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(base, ns))

	d := draftorderModel.New(db)
	d.Email = email
	d.Currency = "usd"
	d.Status = draftorderModel.StatusDraft
	if err := d.Create(); err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	for i := range lines {
		it := draftorderitem.New(db)
		it.DraftOrderId = d.Id()
		it.ProductId = lines[i].ProductId
		it.ProductName = lines[i].ProductName
		it.VariantId = lines[i].VariantId
		it.Quantity = lines[i].Quantity
		it.UnitPriceCents = lines[i].UnitPriceCents
		it.Currency = "usd"
		if err := it.Create(); err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}
	return d
}

// TestComplete_BuildsOrderWithMatchingTotal proves the core flow: create draft +
// add items + complete → a real order with the same items and matching total,
// and the draft flips to complete with a back-reference to the order.
func TestComplete_BuildsOrderWithMatchingTotal(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	d := seedDraft(t, context.Background(), "acme", "buyer@acme.test", []draftorderitem.DraftOrderItem{
		{ProductId: "prod-1", ProductName: "Widget", Quantity: 2, UnitPriceCents: 1500}, // 3000
		{VariantId: "var-9", ProductName: "Gadget", Quantity: 3, UnitPriceCents: 800},   // 2400
	})
	const wantTotal = currency.Cents(5400)

	code, b := callComplete(t, "acme", true, d.Id())
	if code != 201 {
		t.Fatalf("complete status = %d, want 201; body=%s", code, b)
	}

	var o order.Order
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatalf("decode order: %v; body=%s", err, b)
	}
	if o.Total != wantTotal {
		t.Fatalf("order total = %d, want %d", o.Total, wantTotal)
	}
	if o.Subtotal != wantTotal || o.LineTotal != wantTotal {
		t.Fatalf("order subtotal/lineTotal = %d/%d, want %d", o.Subtotal, o.LineTotal, wantTotal)
	}
	if len(o.Items) != 2 {
		t.Fatalf("order items = %d, want 2", len(o.Items))
	}
	if o.Email != "buyer@acme.test" {
		t.Fatalf("order email = %q, want buyer@acme.test", o.Email)
	}

	// Draft is now terminal and points at the order it produced.
	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	reloaded := draftorderModel.New(db)
	if err := reloaded.GetById(d.Id()); err != nil {
		t.Fatalf("reload draft: %v", err)
	}
	if reloaded.Status != draftorderModel.StatusComplete {
		t.Fatalf("draft status = %q, want complete", reloaded.Status)
	}
	if reloaded.OrderId != o.Id() {
		t.Fatalf("draft orderId = %q, want %q", reloaded.OrderId, o.Id())
	}
}

// TestComplete_IdempotentReplay proves completing an already-completed draft
// returns the same order (200) rather than minting a second one.
func TestComplete_IdempotentReplay(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	d := seedDraft(t, context.Background(), "acme", "buyer@acme.test", []draftorderitem.DraftOrderItem{
		{ProductId: "prod-1", Quantity: 1, UnitPriceCents: 1000},
	})

	code1, b1 := callComplete(t, "acme", true, d.Id())
	if code1 != 201 {
		t.Fatalf("complete 1 status = %d; body=%s", code1, b1)
	}
	var o1 order.Order
	_ = json.Unmarshal(b1, &o1)

	code2, b2 := callComplete(t, "acme", true, d.Id())
	if code2 != 200 {
		t.Fatalf("complete replay status = %d, want 200; body=%s", code2, b2)
	}
	var o2 order.Order
	_ = json.Unmarshal(b2, &o2)
	if o1.Id() != o2.Id() {
		t.Fatalf("replay produced a different order: %s vs %s", o1.Id(), o2.Id())
	}

	// Exactly one order exists in the org.
	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	orders := make([]*order.Order, 0, 4)
	if _, err := order.Query(db).GetAll(&orders); err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("orders in org = %d, want 1 (idempotent complete)", len(orders))
	}
}

// TestComplete_NoItems400 proves a draft with no line items cannot be completed.
func TestComplete_NoItems400(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	d := seedDraft(t, context.Background(), "acme", "buyer@acme.test", nil)

	code, b := callComplete(t, "acme", true, d.Id())
	if code != 400 {
		t.Fatalf("complete empty draft = %d, want 400; body=%s", code, b)
	}
}

// TestComplete_NonAdmin403 proves the money action is admin-gated: an
// authenticated non-admin caller is refused before any order is created.
func TestComplete_NonAdmin403(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	d := seedDraft(t, context.Background(), "acme", "buyer@acme.test", []draftorderitem.DraftOrderItem{
		{ProductId: "prod-1", Quantity: 1, UnitPriceCents: 1000},
	})

	code, b := callComplete(t, "acme", false, d.Id()) // authenticated, NOT admin
	if code != 403 {
		t.Fatalf("complete as non-admin = %d, want 403; body=%s", code, b)
	}

	// No order was created.
	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	orders := make([]*order.Order, 0, 4)
	if _, err := order.Query(db).GetAll(&orders); err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("orders in org = %d, want 0 (refused complete created nothing)", len(orders))
	}
}

// TestComplete_CrossTenant404 proves a draft in org acme is a 404 for org beta.
func TestComplete_CrossTenant404(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	d := seedDraft(t, context.Background(), "acme", "buyer@acme.test", []draftorderitem.DraftOrderItem{
		{ProductId: "prod-1", Quantity: 1, UnitPriceCents: 1000},
	})

	code, b := callComplete(t, "beta", true, d.Id()) // caller is org beta, draft belongs to acme
	if code != 404 {
		t.Fatalf("cross-tenant complete = %d, want 404; body=%s", code, b)
	}
}
