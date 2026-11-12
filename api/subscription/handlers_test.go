package subscription

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

	billing "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	submodel "github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// subAPI wires the real exported subscription handlers behind a seed middleware
// that binds org `ns` (Init'd against the ae datastore) into the request the way
// the token-auth group does in production — so subscribe/update/unsubscribe run
// their real org.Namespaced(...) datastore path.
type subAPI struct {
	app *zip.App
	org *organization.Organization
	db  *datastore.Datastore
}

func newSubAPI(ns string) *subAPI {
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
	app.Raw(http.MethodPost, "/subscribe", seed, Subscribe)
	app.Raw(http.MethodGet, "/subscribe/:subscriptionid", seed, GetSubscribe)
	app.Raw(http.MethodPatch, "/subscribe/:subscriptionid", seed, UpdateSubscribe)
	app.Raw(http.MethodDelete, "/subscribe/:subscriptionid", seed, Unsubscribe)

	return &subAPI{app: app, org: org, db: db}
}

func (s *subAPI) do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		r = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (s *subAPI) seedPlan(t *testing.T, price int64) *plan.Plan {
	t.Helper()
	p := plan.New(s.db)
	p.Name = "Pro"
	p.Price = currency.Cents(price)
	p.Currency = currency.USD
	p.Interval = types.Monthly
	p.IntervalCount = 1
	if err := p.Create(); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return p
}

func (s *subAPI) onlySub(t *testing.T) *submodel.Subscription {
	t.Helper()
	subs := make([]*submodel.Subscription, 0)
	if _, err := submodel.Query(s.db).GetAll(&subs); err != nil {
		t.Fatalf("query subs: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("subscription count = %d, want 1", len(subs))
	}
	return subs[0]
}

// TestSubscriptionCRUDLifecycle drives the full create -> read -> update ->
// delete money lifecycle through the real handlers against the real datastore.
func TestSubscriptionCRUDLifecycle(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newSubAPI("acme")
	pln := api.seedPlan(t, 2000)

	// CREATE
	code, body := api.do(t, http.MethodPost, "/subscribe", map[string]any{
		"user":         map[string]any{"email": "alice@acme.test", "username": "alice"},
		"subscription": map[string]any{"planId": pln.Id(), "quantity": 1},
	})
	if code != http.StatusOK {
		t.Fatalf("subscribe status = %d, body=%s", code, body)
	}
	created := api.onlySub(t)
	if created.PlanId != pln.Id() {
		t.Fatalf("created planId = %q, want %q", created.PlanId, pln.Id())
	}
	if created.Quantity != 1 {
		t.Fatalf("created quantity = %d, want 1", created.Quantity)
	}
	id := created.Id()

	// READ
	code, body = api.do(t, http.MethodGet, "/subscribe/"+id, nil)
	if code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", code, body)
	}

	// UPDATE — bump quantity; userId unchanged so the guard passes.
	code, body = api.do(t, http.MethodPatch, "/subscribe/"+id, map[string]any{"quantity": 2})
	if code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", code, body)
	}
	updated := submodel.New(api.db)
	if err := updated.GetById(id); err != nil {
		t.Fatalf("reload updated: %v", err)
	}
	if updated.Quantity != 2 {
		t.Fatalf("updated quantity = %d, want 2", updated.Quantity)
	}

	// DELETE (unsubscribe)
	code, body = api.do(t, http.MethodDelete, "/subscribe/"+id, nil)
	if code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", code, body)
	}
}

// TestSubscribe_PlanDoesNotExist proves subscribe fails (no subscription
// persisted) when the requested plan is unknown.
func TestSubscribe_PlanDoesNotExist(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newSubAPI("acme")

	code, _ := api.do(t, http.MethodPost, "/subscribe", map[string]any{
		"user":         map[string]any{"email": "bob@acme.test", "username": "bob"},
		"subscription": map[string]any{"planId": "plan_missing", "quantity": 1},
	})
	if code != http.StatusInternalServerError {
		t.Fatalf("subscribe with missing plan status = %d, want 500", code)
	}
	subs := make([]*submodel.Subscription, 0)
	if _, err := submodel.Query(api.db).GetAll(&subs); err != nil {
		t.Fatalf("query subs: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("subscription count = %d, want 0 (nothing persisted on plan failure)", len(subs))
	}
}

// TestGetSubscribe_NotFound proves reading an unknown subscription is a 404.
func TestGetSubscribe_NotFound(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	api := newSubAPI("acme")
	code, _ := api.do(t, http.MethodGet, "/subscribe/nope", nil)
	if code != http.StatusNotFound {
		t.Fatalf("get missing status = %d, want 404", code)
	}
}

// The merchant subscription API writes the merchant's customers' rows, which
// share the org's namespace with the org's own Hanzo plan. It never makes a row
// that opens the store or admin: a body naming a Hanzo slug is stored under the
// merchant plan's id, the providers only Hanzo billing writes are refused, and a
// Hanzo subscription row is not its to edit.
func TestTheMerchantAPINeverWritesAHanzoSubscription(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()
	if _, _, err := billing.SeedPlans(tc); err != nil {
		t.Fatalf("seed: %v", err)
	}
	api := newSubAPI("acme")
	own := api.seedPlan(t, 2000)
	own.Slug = "dev"
	if err := own.Update(); err != nil {
		t.Fatal(err)
	}
	code, body := api.do(t, http.MethodPost, "/subscribe", map[string]any{
		"user":         map[string]any{"email": "bob@acme.test", "username": "bob"},
		"subscription": map[string]any{"planId": own.Id(), "quantity": 1},
	})
	if code != http.StatusOK {
		t.Fatalf("subscribe: %d %s", code, body)
	}
	id := api.onlySub(t).Id()
	later := time.Now().AddDate(70, 0, 0).UTC().Format(time.RFC3339)

	code, body = api.do(t, http.MethodPatch, "/subscribe/"+id, map[string]any{
		"planId": "dev", "status": "active", "providerType": "square", "quantity": 1, "current_period_end": later, "periodEnd": later,
	})
	if code != http.StatusOK {
		t.Fatalf("patch: %d %s", code, body)
	}
	got := submodel.New(api.db)
	if err := got.GetById(id); err != nil {
		t.Fatal(err)
	}
	if got.PlanId != own.Id() || billing.Unlocks(api.db.Context, got, time.Now()) {
		t.Fatalf("a merchant row naming the Hanzo slug dev: planId %q, unlocks %v; want the merchant plan's id and nothing opened",
			got.PlanId, billing.Unlocks(api.db.Context, got, time.Now()))
	}

	if code, body = api.do(t, http.MethodPatch, "/subscribe/"+id, map[string]any{"providerType": "manual_gift", "quantity": 1}); code != http.StatusForbidden {
		t.Fatalf("a merchant row made a staff gift: %d %s, want 403", code, body)
	}

	hanzo := submodel.New(api.db)
	hanzo.UserId, hanzo.PlanId, hanzo.ProviderType, hanzo.Status = got.UserId, "go", "square", submodel.Active
	hanzo.Plan.Slug, hanzo.Plan.Price = "go", 800
	hanzo.PeriodStart, hanzo.PeriodEnd = time.Now(), time.Now().AddDate(0, 1, 0)
	if err := hanzo.Create(); err != nil {
		t.Fatal(err)
	}
	if code, body = api.do(t, http.MethodPatch, "/subscribe/"+hanzo.Id(), map[string]any{"quantity": 1, "periodEnd": later}); code != http.StatusForbidden {
		t.Fatalf("the merchant API edited the org's Hanzo plan: %d %s, want 403", code, body)
	}
}
