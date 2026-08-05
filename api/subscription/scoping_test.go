package subscription

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	submodel "github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// subScopeAPI is a self-contained (independent of handlers_test.go) harness that
// binds org `ns` into the request the way the token-auth group does, so
// GetSubscribe runs its real org.Namespaced datastore lookup. It proves the
// per-tenant isolation the money-critical subscription reads depend on.
type subScopeAPI struct {
	app *zip.App
	db  *datastore.Datastore
}

func newSubScopeAPI(ns string) *subScopeAPI {
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
	app.Get("/subscribe/:subscriptionid", seed, GetSubscribe)

	return &subScopeAPI{app: app, db: db}
}

func (a *subScopeAPI) get(t *testing.T, id string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/subscribe/"+id, nil)
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatalf("GET /subscribe/%s: %v", id, err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.StatusCode
}

func (a *subScopeAPI) seedSub(t *testing.T) *submodel.Subscription {
	t.Helper()
	sub := submodel.New(a.db)
	sub.PlanId = "plan-x"
	sub.UserId = "user-1"
	sub.Quantity = 1
	sub.Status = submodel.Active
	if err := sub.Create(); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	return sub
}

// TestSubscription_OrgScoped proves a subscription created in org "acme" is
// readable inside "acme" but INVISIBLE to org "other" — a tenant can never read
// another tenant's subscription by guessing its id.
func TestSubscription_OrgScoped(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	acme := newSubScopeAPI("acme")
	sub := acme.seedSub(t)

	if code := acme.get(t, sub.Id()); code != http.StatusOK {
		t.Fatalf("owner read = %d, want 200", code)
	}

	other := newSubScopeAPI("other")
	if code := other.get(t, sub.Id()); code != http.StatusNotFound {
		t.Fatalf("cross-org read = %d, want 404 (subscription must be tenant-scoped)", code)
	}
}
