package producttaxonomy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/productoption"
	"github.com/hanzoai/commerce/models/productoptionvalue"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// wire builds a zip app with org+context injected, then calls Route. It asserts
// Route does NOT panic — the sibling-wildcard risk: rest's default
// /:productoptionid route and the custom /:productoptionid/values subroute
// must share the SAME wildcard name or the router panics at wiring.
func wire(t *testing.T, base context.Context, ns string) *zip.App {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	r := app.Use(zip.H(func(c *zip.Ctx) error {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(base)
		return c.Next()
	}))
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("producttaxonomy.Route panicked at wiring (sibling wildcard?): %v", rec)
		}
	}()
	Route(r) // no token middleware — exercise the handlers directly
	return app
}

func TestRoute_WiresAndOptionValuesSubrouteWorks(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	base := nscontext.WithNamespace(context.Background(), "acme")
	eng := wire(t, base, "acme")

	// Seed an option in the acme namespace.
	db := datastore.New(base)
	opt := productoption.New(db)
	opt.Title = "Size"
	opt.ProductId = "prod_1"
	if err := opt.Create(); err != nil {
		t.Fatalf("seed option: %v", err)
	}

	// POST a value through the wired router (proves the :productoptionid param
	// plumbs from the URL to the handler's Param lookup).
	req := httptest.NewRequest(http.MethodPost, "/product-option/"+opt.Id()+"/values",
		strings.NewReader(`{"value":"Large"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := eng.Test(req)
	if err != nil {
		t.Fatalf("add value request: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("add value status = %d, want 201; body=%s", resp.StatusCode, body)
	}

	// GET the values back.
	req2 := httptest.NewRequest(http.MethodGet, "/product-option/"+opt.Id()+"/values", nil)
	resp2, err := eng.Test(req2)
	if err != nil {
		t.Fatalf("list values request: %v", err)
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("list values status = %d, want 200; body=%s", resp2.StatusCode, body2)
	}
	var vals []*productoptionvalue.ProductOptionValue
	if err := json.Unmarshal(body2, &vals); err != nil {
		t.Fatalf("values not JSON: %s", body2)
	}
	if len(vals) != 1 || vals[0].Value != "Large" || vals[0].OptionId != opt.Id() {
		t.Fatalf("values = %+v, want one {Large, %s}", vals, opt.Id())
	}
}
