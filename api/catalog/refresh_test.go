package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestAdminRoute_AddressesASlashBearingSlug drives the REAL AdminRoute wiring.
// A model's slug IS its callable id and those contain a slash, so with a :slug
// segment param every model row would be listed and un-editable — a catalog
// whose own admin surface cannot address most of its rows, which is the class of
// defect this whole convergence removes. The single-segment case is checked in
// the same pass, because an infra tier must keep working byte-for-byte.
func TestAdminRoute_AddressesASlashBearingSlug(t *testing.T) {
	seen := make(chan string, 4)
	app := zip.New(zip.Config{DisableStartupMessage: true})
	AdminRoute(app.Group("/v1"), func(c *zip.Ctx) error {
		if c.Method() == http.MethodPut {
			seen <- entrySlug(c)
			return c.JSON(200, map[string]string{"slug": entrySlug(c)})
		}
		return c.Next()
	})
	h := adaptor.FiberApp(app.Fiber())

	for _, want := range []string{
		"cloud-basic",             // an infra tier
		"anthropic/claude-opus-5", // a model
		"~openai/gpt-latest",      // a floating alias
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/v1/catalog/entries/"+want, nil))
		if w.Code != 200 {
			t.Fatalf("PUT %s → %d, want 200", want, w.Code)
		}
		if got := <-seen; got != want {
			t.Fatalf("addressed slug = %q, want %q", got, want)
		}
	}
}

// TestRefreshModels_RequiresSuperAdmin: the refresh writes platform-global data,
// so an org-level admin must not reach it — the same boundary every other
// catalog mutation holds. Refused BEFORE any upstream call, so an unauthorized
// request cannot even cause outbound traffic.
func TestRefreshModels_RequiresSuperAdmin(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	for _, claims := range []*auth.IAMClaims{nil, {Owner: "acme", IsAdmin: true}} {
		code, _ := callCatalog(t, claims, http.MethodPost, "/v1/catalog/models/refresh", nil, RefreshModels)
		if code != 403 {
			t.Fatalf("claims %+v → %d, want 403", claims, code)
		}
	}
}
