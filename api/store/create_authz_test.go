// Copyright © 2026 Hanzo AI. MIT License.

package store

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/commerce/util/test/zipclient"
)

// newStoreAPI wires the store CRUD exactly as the /v1 bundle does — the
// gateway-trust IAMTokenRequired group middleware (which turns the minted
// X-Org-Id / X-User-IsAdmin headers into the request's org + permissions) in
// front of storeApi.Route(api, tokenRequired). This is the real gate POST
// /v1/store passes through.
func newStoreAPI(ctx ae.Context) *zipclient.Client {
	cl := zipclient.New(ctx)
	cl.Use(iammiddleware.IAMTokenRequired())
	Route(cl.Router, middleware.TokenRequired())
	return cl
}

func postStore(cl *zipclient.Client, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/store", strings.NewReader(`{"name":"onboarding-store"}`))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return cl.Do(req)
}

// TestCreateStore_OrgAdminOwnOrg is onboarding step 1: a gateway-verified ORG
// admin (X-User-IsAdmin=="true") creating a store in ITS OWN org (X-Org-Id) must
// succeed. Before the fix the org owner's permissions carried no WriteStore bit,
// so rest.CheckPermissions("create") 403'd and self-serve onboarding died here.
func TestCreateStore_OrgAdminOwnOrg(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	cl := newStoreAPI(ctx)

	w := postStore(cl, map[string]string{
		iammiddleware.HeaderUserIsAdmin: "true",
		iammiddleware.HeaderUserOwner:   "acme", // HOME org
		"X-Org-Id":                      "acme", // EFFECTIVE org — equal, so the grant applies
		"X-User-Id":                     "user-acme-owner",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("org admin create store in own org: got %d, want 201\nbody: %s", w.Code, w.Body.String())
	}
}

// TestCreateStore_NonAdminRefused: an authenticated principal that is NOT an org
// admin (no X-User-IsAdmin) holds no WriteStore bit, so it cannot create a store
// even in its own org.
func TestCreateStore_NonAdminRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	cl := newStoreAPI(ctx)

	w := postStore(cl, map[string]string{
		"X-Org-Id":  "acme",
		"X-User-Id": "user-acme-member",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin create store: got %d, want 403\nbody: %s", w.Code, w.Body.String())
	}
}

// TestCreateStore_CrossOrgRefused proves the grant can never cross tenants — the
// ADVERSARIAL case: the caller carries X-User-IsAdmin=="true" AND a HOME org
// (X-User-Owner="acme") but the EFFECTIVE org (X-Org-Id="other") is a DIFFERENT
// tenant. Commerce must NOT apply orgAdminGrant when home != effective (it enforces
// the coupling itself via orgAdminHomeMatches, rather than trusting the gateway's
// binding by comment), so the create is refused — an org-switched principal can
// never gain merchant authority over a foreign org.
func TestCreateStore_CrossOrgRefused(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()
	cl := newStoreAPI(ctx)

	w := postStore(cl, map[string]string{
		iammiddleware.HeaderUserIsAdmin: "true",  // admin flag present…
		iammiddleware.HeaderUserOwner:   "acme",  // …of the HOME org acme…
		"X-Org-Id":                      "other", // …but acting in a DIFFERENT (victim) org
		"X-User-Id":                     "user-acme-owner",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-org create store (home!=effective): got %d, want 403\nbody: %s", w.Code, w.Body.String())
	}
}
