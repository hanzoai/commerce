package cart

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
	cartmodel "github.com/hanzoai/commerce/models/cart"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// cartAPI wires the real Set/Discard handlers behind a seed middleware that
// binds org `ns` the way the token-auth group does in production, so the
// handlers run their real org.Namespaced(...) datastore path.
type cartAPI struct {
	app *zip.App
	org *organization.Organization
	db  *datastore.Datastore
}

func newCartAPI(ns string) *cartAPI {
	base := nscontext.WithNamespace(context.Background(), ns)
	db := datastore.New(base)
	org := organization.New(db)
	org.Name = ns

	app := zip.New(zip.Config{DisableStartupMessage: true})
	seed := func(c *zip.Ctx) error {
		c.SetContext(base)
		c.Locals("organization", org)
		return c.Next()
	}
	app.Post("/:cartid/set", seed, Set)
	app.Post("/:cartid/discard", seed, Discard)

	return &cartAPI{app: app, org: org, db: db}
}

func (a *cartAPI) do(t *testing.T, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Fiber().Test(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (a *cartAPI) seedCart(t *testing.T) *cartmodel.Cart {
	t.Helper()
	car := cartmodel.New(a.db)
	if err := car.Create(); err != nil {
		t.Fatalf("create cart: %v", err)
	}
	return car
}

func (a *cartAPI) seedProduct(t *testing.T) *product.Product {
	t.Helper()
	p := product.New(a.db)
	p.Name = "Widget"
	p.Slug = "widget"
	p.SKU = "WIDGET-1"
	if err := p.Create(); err != nil {
		t.Fatalf("create product: %v", err)
	}
	return p
}

func (a *cartAPI) reload(t *testing.T, id string) *cartmodel.Cart {
	t.Helper()
	car := cartmodel.New(a.db)
	if err := car.GetById(id); err != nil {
		t.Fatalf("reload cart: %v", err)
	}
	return car
}

// TestSet_AddThenUpdateThenRemove drives the line-item lifecycle: adding a
// product creates a line item at the requested quantity; setting it again
// replaces the quantity (never duplicates the line); quantity 0 removes it.
func TestSet_AddThenUpdateThenRemove(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCartAPI("acme")
	car := api.seedCart(t)
	prod := api.seedProduct(t)

	// ADD qty 2
	code, body := api.do(t, "/"+car.Id()+"/set", map[string]any{
		"productId": prod.Id(), "quantity": 2,
	})
	if code != http.StatusOK {
		t.Fatalf("set add status = %d, body=%s", code, body)
	}
	got := api.reload(t, car.Id())
	if len(got.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(got.Items))
	}
	if got.Items[0].Quantity != 2 {
		t.Fatalf("qty = %d, want 2", got.Items[0].Quantity)
	}
	if got.Items[0].ProductId != prod.Id() {
		t.Fatalf("line productId = %q, want %q", got.Items[0].ProductId, prod.Id())
	}

	// UPDATE same product to qty 5 — must replace, not append.
	code, body = api.do(t, "/"+car.Id()+"/set", map[string]any{
		"productId": prod.Id(), "quantity": 5,
	})
	if code != http.StatusOK {
		t.Fatalf("set update status = %d, body=%s", code, body)
	}
	got = api.reload(t, car.Id())
	if len(got.Items) != 1 {
		t.Fatalf("items after update = %d, want 1 (no duplicate line)", len(got.Items))
	}
	if got.Items[0].Quantity != 5 {
		t.Fatalf("qty after update = %d, want 5", got.Items[0].Quantity)
	}

	// REMOVE via qty 0.
	code, body = api.do(t, "/"+car.Id()+"/set", map[string]any{
		"productId": prod.Id(), "quantity": 0,
	})
	if code != http.StatusOK {
		t.Fatalf("set remove status = %d, body=%s", code, body)
	}
	got = api.reload(t, car.Id())
	if len(got.Items) != 0 {
		t.Fatalf("items after remove = %d, want 0", len(got.Items))
	}
}

// TestSet_NoItemSpecified: a set with neither product nor variant is a 400.
func TestSet_NoItemSpecified(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCartAPI("acme")
	car := api.seedCart(t)

	code, body := api.do(t, "/"+car.Id()+"/set", map[string]any{"quantity": 1})
	if code != http.StatusBadRequest {
		t.Fatalf("set with no item = %d, want 400; body=%s", code, body)
	}
}

// TestSet_UnknownCart: setting on a nonexistent cart is a 404.
func TestSet_UnknownCart(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCartAPI("acme")
	code, _ := api.do(t, "/nope/set", map[string]any{"productId": "x", "quantity": 1})
	if code != http.StatusNotFound {
		t.Fatalf("set unknown cart = %d, want 404", code)
	}
}

// TestDiscard marks a cart discarded and is idempotent-safe to read back.
func TestDiscard(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCartAPI("acme")
	car := api.seedCart(t)

	code, body := api.do(t, "/"+car.Id()+"/discard", nil)
	if code != http.StatusOK {
		t.Fatalf("discard status = %d, body=%s", code, body)
	}
	got := api.reload(t, car.Id())
	if got.Status != cartmodel.Discarded {
		t.Fatalf("status = %q, want %q", got.Status, cartmodel.Discarded)
	}
}

// TestDiscard_UnknownCart: discarding a nonexistent cart is a 404.
func TestDiscard_UnknownCart(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newCartAPI("acme")
	code, _ := api.do(t, "/nope/discard", nil)
	if code != http.StatusNotFound {
		t.Fatalf("discard unknown cart = %d, want 404", code)
	}
}
