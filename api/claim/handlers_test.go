package claim

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
	claimModel "github.com/hanzoai/commerce/models/claim"
	"github.com/hanzoai/commerce/models/lineitem"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/refund"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// seed installs the org + IAM claim so middleware.GetOrganization(c) and
// org.Namespaced(c.Context()) resolve to org `ns`'s namespace — the exact
// production plumbing. admin selects whether the injected claim authorizes the
// admin-gated accept action.
func seed(ns string, admin bool) zip.Handler {
	return func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
		c.Locals("iam_authenticated", true)
		c.Locals("iam_claims", &auth.IAMClaims{Owner: ns, IsAdmin: admin})
		return c.Next()
	}
}

// callCreate drives Create for org ns.
func callCreate(t *testing.T, ns string, body []byte) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(http.MethodPost, "/claim", seed(ns, true), Create)
	return do(t, app, http.MethodPost, "/claim", body)
}

// callAction drives an id-scoped action (accept/reject) for org ns.
func callAction(t *testing.T, ns string, admin bool, id, action string, h zip.Handler) (int, []byte) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(http.MethodPost, "/claim/:claimid/"+action, seed(ns, admin), h)
	return do(t, app, http.MethodPost, "/claim/"+id+"/"+action, nil)
}

func do(t *testing.T, app *zip.App, method, path string, body []byte) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// seedOrder installs an order with two lines in org ns.
//
//	widget: 3 @ 1000c   gadget: 2 @ 2500c
func seedOrder(t *testing.T, ns string) *order.Order {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	o := order.New(db)
	o.Currency = "usd"
	o.Total = 8000
	o.Paid = 8000
	o.Items = []lineitem.LineItem{
		lineWith("widget", 3, 1000),
		lineWith("gadget", 2, 2500),
	}
	if err := o.Create(); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return o
}

func lineWith(productId string, qty int, price int64) lineitem.LineItem {
	var li lineitem.LineItem
	li.ProductId = productId
	li.Quantity = qty
	li.Price = currency.Cents(price)
	return li
}

// mustCreateClaim files a claim over the wire and returns it.
func mustCreateClaim(t *testing.T, ns, orderId, resolution string, items []itemRequest) *claimModel.Claim {
	t.Helper()
	body, _ := json.Marshal(createRequest{OrderId: orderId, Resolution: resolution, Items: items})
	code, b := callCreate(t, ns, body)
	if code != 201 {
		t.Fatalf("create claim status = %d; body=%s", code, b)
	}
	var cl claimModel.Claim
	if err := json.Unmarshal(b, &cl); err != nil {
		t.Fatalf("decode claim: %v", err)
	}
	return &cl
}

// TestAccept_Replace_ComputesAmount proves the money math: a claim for 2 widgets
// (@1000) + 1 gadget (@2500) settles to 4500c by a replacement order, and the
// order's Refunded is untouched.
func TestAccept_Replace_ComputesAmount(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "widget", Quantity: 2, Reason: "damaged"},
		{ItemId: "gadget", Quantity: 1, Reason: "wrong_item"},
	})

	code, b := callAction(t, "acme", true, cl.Id(), "accept", Accept)
	if code != 200 {
		t.Fatalf("accept status = %d; body=%s", code, b)
	}
	var resp acceptResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		t.Fatalf("decode accept: %v", err)
	}
	if resp.AmountCents != 4500 {
		t.Fatalf("settled amount = %d, want 4500", resp.AmountCents)
	}
	if resp.ReplacementOrderId == "" {
		t.Fatalf("expected a replacement order id")
	}
	if resp.Claim.Status != claimModel.StatusAccepted {
		t.Fatalf("claim status = %q, want accepted", resp.Claim.Status)
	}

	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	fresh := order.New(db)
	if err := fresh.GetById(o.Id()); err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if fresh.Refunded != 0 {
		t.Fatalf("order refunded = %d, want 0", fresh.Refunded)
	}
}

// TestAccept_Idempotent proves a second accept returns the SAME replacement order
// and builds no second one.
func TestAccept_Idempotent(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "widget", Quantity: 1, Reason: "missing"},
	})

	_, b1 := callAction(t, "acme", true, cl.Id(), "accept", Accept)
	var r1 acceptResponse
	_ = json.Unmarshal(b1, &r1)

	code2, b2 := callAction(t, "acme", true, cl.Id(), "accept", Accept)
	if code2 != 200 {
		t.Fatalf("re-accept status = %d; body=%s", code2, b2)
	}
	var r2 acceptResponse
	_ = json.Unmarshal(b2, &r2)

	if r1.ReplacementOrderId == "" || r1.ReplacementOrderId != r2.ReplacementOrderId {
		t.Fatalf("replay produced a different replacement: %q vs %q", r1.ReplacementOrderId, r2.ReplacementOrderId)
	}
}

// TestAccept_CannotClaimMoreThanOrdered proves over-claiming a line is rejected
// (422) before any order is created.
func TestAccept_CannotClaimMoreThanOrdered(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "widget", Quantity: 99, Reason: "damaged"}, // ordered only 3
	})

	code, b := callAction(t, "acme", true, cl.Id(), "accept", Accept)
	if code != 422 {
		t.Fatalf("over-claim accept status = %d, want 422; body=%s", code, b)
	}

	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	fresh := order.New(db)
	_ = fresh.GetById(o.Id())
	if int64(fresh.Refunded) != 0 {
		t.Fatalf("order refunded = %d after refused over-claim, want 0", fresh.Refunded)
	}
	reload := claimModel.New(db)
	_ = reload.GetById(cl.Id())
	if reload.Status != claimModel.StatusPending {
		t.Fatalf("claim status = %q after refused accept, want pending", reload.Status)
	}
}

// TestAccept_NonAdmin_403 proves accept rejects a non-admin.
func TestAccept_NonAdmin_403(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "widget", Quantity: 1, Reason: "damaged"},
	})

	code, b := callAction(t, "acme", false, cl.Id(), "accept", Accept) // authenticated, NOT admin
	if code != 403 {
		t.Fatalf("non-admin accept status = %d, want 403; body=%s", code, b)
	}
}

// TestClaim_CrossTenant404 proves a claim in acme is a 404 for org beta.
func TestClaim_CrossTenant404(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "widget", Quantity: 1, Reason: "damaged"},
	})

	code, b := callAction(t, "beta", true, cl.Id(), "accept", Accept) // caller is beta
	if code != 404 {
		t.Fatalf("cross-tenant accept status = %d, want 404; body=%s", code, b)
	}
}

// TestReject transitions a pending claim to rejected and is idempotent; it never
// touches the order.
func TestReject(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", []itemRequest{
		{ItemId: "gadget", Quantity: 1, Reason: "damaged"},
	})

	code, b := callAction(t, "acme", true, cl.Id(), "reject", Reject)
	if code != 200 {
		t.Fatalf("reject status = %d; body=%s", code, b)
	}
	var got claimModel.Claim
	_ = json.Unmarshal(b, &got)
	if got.Status != claimModel.StatusRejected {
		t.Fatalf("status = %q, want rejected", got.Status)
	}

	// A rejected claim cannot then be accepted.
	code2, _ := callAction(t, "acme", true, cl.Id(), "accept", Accept)
	if code2 != 409 {
		t.Fatalf("accept-after-reject status = %d, want 409", code2)
	}
}

// TestClaim_NeverRefunds proves a claim is settled only by replacement: one asking
// for a refund is refused at create, one filed without a resolution is a
// replacement, and a refund claim already on file is refused at accept — no refund
// row, and the order's Refunded untouched.
func TestClaim_NeverRefunds(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	o := seedOrder(t, "acme")
	items := []itemRequest{{ItemId: "widget", Quantity: 1, Reason: "damaged"}}
	body, _ := json.Marshal(createRequest{OrderId: o.Id(), Resolution: "refund", Items: items})
	if code, b := callCreate(t, "acme", body); code != 400 {
		t.Fatalf("create refund claim status = %d, want 400; body=%s", code, b)
	}
	if cl := mustCreateClaim(t, "acme", o.Id(), "", items); cl.Resolution != claimModel.ResolutionReplace {
		t.Fatalf("default resolution = %q, want replace", cl.Resolution)
	}

	db := datastore.New(nscontext.WithNamespace(context.Background(), "acme"))
	cl := mustCreateClaim(t, "acme", o.Id(), "replace", items)
	onFile := claimModel.New(db)
	if err := onFile.GetById(cl.Id()); err != nil {
		t.Fatalf("load claim: %v", err)
	}
	onFile.Resolution = "refund"
	if err := onFile.Update(); err != nil {
		t.Fatalf("file a refund claim: %v", err)
	}
	if code, b := callAction(t, "acme", true, cl.Id(), "accept", Accept); code != 422 {
		t.Fatalf("accept refund claim status = %d, want 422; body=%s", code, b)
	}
	fresh := order.New(db)
	if err := fresh.GetById(o.Id()); err != nil || fresh.Refunded != 0 {
		t.Fatalf("order refunded = %d (%v), want 0", fresh.Refunded, err)
	}
	rows := make([]*refund.Refund, 0)
	if _, err := refund.Query(db).GetAll(&rows); err != nil || len(rows) != 0 {
		t.Fatalf("refund rows = %d (%v), want none", len(rows), err)
	}
}
