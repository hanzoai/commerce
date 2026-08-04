package billing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/util/bit"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/test/ae"
)

// The portal payment-method pair is now REACHABLE — the list is what a proxying
// host reads to render saved cards, and the detach is what removes one. Both key
// their finer scope on a value the CALLER supplies (?customerId, :id), so the only
// thing between org B and org A's card is the namespace the org resolves to.
//
// api/billing/idor_test.go's sibling suite proves the INTRA-org half (one member
// may not reach another member's card inside one org). This proves the CROSS-org
// half, and it proves it for the PRIVILEGED caller too: the service token carries
// the Admin bit, which bypasses callerMayReachBillingSubject by design, so the
// namespace is the ONLY control left on that path. If it ever stopped scoping,
// every intra-org test here would still pass.
//
// Same shape as TestDownloadInvoicePDF_TenantIsolation: seed in org A's namespace,
// then hand org A's id to org B and require a miss.

// pmSeed scopes a request to org `ns` the way the gateway does — the namespace
// GetOrganization and org.Namespaced read. `admin` gives the caller the Admin
// permission bit a verified COMMERCE_SERVICE_TOKEN arrives with.
func pmSeed(ns string, admin bool) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		if admin {
			c.Locals("permissions", bit.Field(permission.Admin|permission.Live))
		}
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
	}
}

// seedCard writes a saved card into org `ns` owned by that org's billing subject
// (the org slug — exactly what orgBillingKey resolves) and returns its id.
func seedCard(t *testing.T, ns string) string {
	t.Helper()
	db := datastore.New(nscontext.WithNamespace(context.Background(), ns))
	pm := paymentmethod.New(db)
	pm.CustomerId = ns
	pm.UserId = ns
	pm.Type = "card"
	pm.ProviderRef = "ccof:tenant-test"
	pm.Card = &paymentmethod.CardDetails{Brand: "visa", Last4: "4242", ExpMonth: 12, ExpYear: 2032}
	if err := pm.Create(); err != nil {
		t.Fatalf("seed card in %s: %v", ns, err)
	}
	return pm.Id()
}

// portalList drives GET /v1/billing/portal/methods as `ns` asking for
// `customerId`, and returns the status plus the ids it exposed.
func portalList(t *testing.T, ns, customerId string, admin bool) (int, []string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/portal/methods?customerId="+customerId, nil)
	w := driveSeeded(pmSeed(ns, admin), "/v1/billing/portal/methods", req, PortalPaymentMethods)
	raw, _ := io.ReadAll(w.Body)
	var got []map[string]any
	_ = json.Unmarshal(raw, &got)
	ids := make([]string, 0, len(got))
	for _, m := range got {
		if id, ok := m["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return w.StatusCode, ids
}

// portalDetach drives DELETE /v1/billing/portal/methods/:id as `ns`.
func portalDetach(t *testing.T, ns, id string, admin bool) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/v1/billing/portal/methods/"+id, nil)
	w := driveSeeded(pmSeed(ns, admin), "/v1/billing/portal/methods/:id", req, DetachPaymentMethod)
	return w.StatusCode
}

// TestPortalPaymentMethods_TenantIsolation: org B cannot LIST org A's saved cards,
// even by naming org A's customerId — and not even holding the service token.
func TestPortalPaymentMethods_TenantIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	idA := seedCard(t, "pm-org-a")

	// org A sees its OWN card.
	if status, ids := portalList(t, "pm-org-a", "pm-org-a", false); status != 200 || len(ids) != 1 || ids[0] != idA {
		t.Fatalf("org A own list: status=%d ids=%v, want 200 with [%s]", status, ids, idA)
	}

	// org B, naming org A's customerId, sees NOTHING — as a member and as a
	// service token. The forged customerId selects a subject inside org B's
	// namespace, where it owns nothing.
	for _, admin := range []bool{false, true} {
		status, ids := portalList(t, "pm-org-b", "pm-org-a", admin)
		if status != 200 || len(ids) != 0 {
			t.Fatalf("CROSS-TENANT LEAK: org B (admin=%v) listing customerId=pm-org-a got status=%d ids=%v, want 200 with []",
				admin, status, ids)
		}
	}
}

// TestDetachPaymentMethod_TenantIsolation: org B cannot DELETE org A's saved card
// by id — the card survives — while org A can delete its own.
func TestDetachPaymentMethod_TenantIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	idA := seedCard(t, "detach-org-a")

	// org B handed org A's id: 404, and the card is still there afterwards.
	// Both profiles, because Admin bypasses the intra-org owner guard.
	for _, admin := range []bool{false, true} {
		if status := portalDetach(t, "detach-org-b", idA, admin); status != 404 {
			t.Fatalf("CROSS-TENANT DELETE: org B (admin=%v) detaching org A's card %s got status=%d, want 404",
				admin, idA, status)
		}
		if status, ids := portalList(t, "detach-org-a", "detach-org-a", false); status != 200 || len(ids) != 1 {
			t.Fatalf("CROSS-TENANT DELETE: org A's card is gone after org B (admin=%v) tried to detach it (status=%d ids=%v)",
				admin, status, ids)
		}
	}

	// org A detaches its OWN card: 200, and the list is empty afterwards.
	if status := portalDetach(t, "detach-org-a", idA, false); status != 200 {
		t.Fatalf("org A own detach: status=%d, want 200", status)
	}
	if status, ids := portalList(t, "detach-org-a", "detach-org-a", false); status != 200 || len(ids) != 0 {
		t.Fatalf("after own detach: status=%d ids=%v, want 200 with []", status, ids)
	}
}
