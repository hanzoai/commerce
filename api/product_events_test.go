package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/events"
)

// driveProduct runs fn inside a real product request so c.Method / c.Param / c.Body
// resolve exactly as they do in production. Both the collection path and the item path
// (/:productid) hit the same handler, so a param-carrying path populates c.Param.
func driveProduct(t *testing.T, method, path, body string, fn func(*zip.Ctx)) {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	h := func(c *zip.Ctx) error {
		fn(c)
		return c.NoContent(204)
	}
	app.All("/v1/product", h)
	app.All("/v1/product/:productid", h)

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if _, err := app.Test(req); err != nil {
		t.Fatalf("test request: %v", err)
	}
}

// productPath maps an optional item param to the request path driveProduct routes.
func productPath(param string) string {
	if param == "" {
		return "/v1/product"
	}
	return "/v1/product/" + param
}

// writeSubject classifies a product REST request into the catalog event it should emit.
func TestWriteSubject(t *testing.T) {
	cases := []struct {
		method, param, want string
	}{
		{http.MethodPost, "", events.SubjectProductCreated},   // create on the collection
		{http.MethodPut, "abc", events.SubjectProductUpdated}, // replace an item
		{http.MethodPatch, "abc", events.SubjectProductUpdated},
		{http.MethodPost, "abc", ""}, // POST /:id is the ambiguous method-override — skip
		{http.MethodGet, "abc", ""},
		{http.MethodGet, "", ""},
		{http.MethodDelete, "abc", ""},
	}
	for _, tc := range cases {
		var got string
		driveProduct(t, tc.method, productPath(tc.param), "", func(c *zip.Ctx) {
			got = writeSubject(c)
		})
		if got != tc.want {
			t.Errorf("writeSubject(%s param=%q) = %q, want %q", tc.method, tc.param, got, tc.want)
		}
	}
}

// peekProductBody extracts the slug/name/sku from the request buffer WITHOUT consuming it,
// so the CRUD handler downstream still decodes the same c.Body().
func TestPeekProductBodyRestoresBody(t *testing.T) {
	const raw = `{"slug":"valentina","name":"Valentina","sku":"SKU-1","price":100}`
	var b productBody
	var afterBody string
	driveProduct(t, http.MethodPost, "/v1/product", raw, func(c *zip.Ctx) {
		b = peekProductBody(c)
		afterBody = string(c.Body()) // c.Body() stays intact for the downstream handler
	})
	if b.Slug != "valentina" || b.Name != "Valentina" || b.SKU != "SKU-1" {
		t.Fatalf("peek wrong: %+v", b)
	}
	if afterBody != raw {
		t.Fatalf("body not intact for handler: got %q want %q", afterBody, raw)
	}
}

func TestPeekProductBodyToleratesGarbage(t *testing.T) {
	var b productBody
	driveProduct(t, http.MethodPost, "/v1/product", "not json", func(c *zip.Ctx) {
		b = peekProductBody(c)
	})
	if b.Slug != "" {
		t.Errorf("garbage body must yield empty slug, got %+v", b)
	}
	// A whitespace-only slug is trimmed to empty (no event fires for it).
	driveProduct(t, http.MethodPost, "/v1/product", `{"slug":"  "}`, func(c *zip.Ctx) {
		b = peekProductBody(c)
	})
	if b.Slug != "" {
		t.Errorf("blank slug must trim to empty, got %q", b.Slug)
	}
}

// eventProductID prefers the URL item param (update), else the id the create handler put
// in the Location header, else "".
func TestEventProductID(t *testing.T) {
	var got string
	driveProduct(t, http.MethodPut, "/v1/product/prod_9", "", func(c *zip.Ctx) {
		got = eventProductID(c)
	})
	if got != "prod_9" {
		t.Errorf("update id = %q, want prod_9", got)
	}

	driveProduct(t, http.MethodPost, "/v1/product", "", func(c *zip.Ctx) {
		c.SetHeader("Location", "/v1/product/prod_created")
		got = eventProductID(c)
	})
	if got != "prod_created" {
		t.Errorf("create id from Location = %q, want prod_created", got)
	}

	driveProduct(t, http.MethodPost, "/v1/product", "", func(c *zip.Ctx) {
		got = eventProductID(c)
	})
	if got != "" {
		t.Errorf("no param/location = %q, want empty", got)
	}
}
