// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package store

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/query"
	"github.com/hanzoai/commerce/db"
	"github.com/hanzoai/commerce/models/organization"
	commercestore "github.com/hanzoai/commerce/models/store"
)

// withResolver installs the SAME per-org DB resolver the production commerce
// bootstrap installs (datastore.SetOrgDBResolver(app.DB.Org)), so
// datastore.NewNamespaced routes each org to its OWN physical store — the wiring
// the getCurrent fix depends on. Mirrors test/perorg.
func withResolver(t *testing.T) func() {
	t.Helper()
	dir, err := os.MkdirTemp("", "storecurrent-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := db.DefaultConfig()
	cfg.DataDir = dir
	cfg.EnableVectorSearch = false
	cfg.EnableDatastore = false
	mgr, err := db.NewManager(cfg)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("NewManager: %v", err)
	}
	sys, err := mgr.Org("system")
	if err != nil {
		mgr.Close()
		os.RemoveAll(dir)
		t.Fatalf("Org(system): %v", err)
	}
	datastore.SetDefaultDB(sys)
	query.SetDefaultDB(sys)
	datastore.SetOrgDBResolver(mgr.Org)

	return func() {
		datastore.SetOrgDBResolver(nil)
		datastore.SetDefaultDB(nil)
		query.SetDefaultDB(nil)
		mgr.Close()
		os.RemoveAll(dir)
	}
}

type storeResp struct {
	Store struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"store"`
}

// callCurrent drives the real getCurrent handler with the given org bound in
// context exactly as middleware.TokenRequired does (c.Set("organization", …)),
// and returns the decoded response. An empty org omits the binding.
func callCurrent(t *testing.T, org string) storeResp {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Get("/v1/store/current",
		func(c *zip.Ctx) error {
			if org != "" {
				c.Locals("organization", &organization.Organization{Name: org})
			}
			return c.Next()
		},
		getCurrent,
	)
	resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, "/v1/store/current", nil))
	if err != nil {
		t.Fatalf("org %q: Test: %v", org, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("org %q: getCurrent status = %d, want 200", org, resp.StatusCode)
	}
	var out storeResp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("org %q: decode body %q: %v", org, string(raw), err)
	}
	return out
}

// TestGetCurrentProvisionsOrgScopedStore is the round-trip proof: an authenticated
// org's GET /v1/store/current resolves (and lazily provisions) a REAL org-scoped
// store id — never the phantom shared "default" — the id the content storefront
// edge needs to publish product imagery.
func TestGetCurrentProvisionsOrgScopedStore(t *testing.T) {
	defer withResolver(t)()

	karma := callCurrent(t, "karma")
	if karma.Store.ID == "" || karma.Store.ID == "default" {
		t.Fatalf("karma store id = %q, want a real provisioned id (not empty/\"default\")", karma.Store.ID)
	}
	if karma.Store.Slug != commercestore.DefaultSlug {
		t.Fatalf("karma store slug = %q, want %q", karma.Store.Slug, commercestore.DefaultSlug)
	}

	karma2 := callCurrent(t, "karma")
	if karma2.Store.ID != karma.Store.ID {
		t.Fatalf("second GET /store/current provisioned a new store: %q != %q", karma2.Store.ID, karma.Store.ID)
	}

	other := callCurrent(t, "acme")
	if other.Store.ID == "" || other.Store.ID == "default" {
		t.Fatalf("acme store id = %q, want a real provisioned id", other.Store.ID)
	}
	if other.Store.ID == karma.Store.ID {
		t.Fatalf("CROSS-TENANT LEAK: acme resolved karma's store id %q", karma.Store.ID)
	}
}

// TestGetCurrentNoOrgFallsBackToDefault proves the endpoint degrades cleanly (no
// panic, no 500) when reached with no org in context.
func TestGetCurrentNoOrgFallsBackToDefault(t *testing.T) {
	defer withResolver(t)()

	resp := callCurrent(t, "")
	if resp.Store.ID != "default" {
		t.Fatalf("no-org store id = %q, want \"default\"", resp.Store.ID)
	}
}

// TestEnsureDefaultIsIdempotent proves the canonical provisioning primitive is
// idempotent at the datastore layer: repeated calls in one org's namespace return
// the SAME store, and a second org gets a distinct one.
func TestEnsureDefaultIsIdempotent(t *testing.T) {
	defer withResolver(t)()

	ctxA := (&organization.Organization{Name: "org-a"}).Namespaced(context.Background())
	a1, err := commercestore.EnsureDefault(datastore.NewNamespaced(ctxA))
	if err != nil {
		t.Fatalf("EnsureDefault org-a: %v", err)
	}
	a2, err := commercestore.EnsureDefault(datastore.NewNamespaced(ctxA))
	if err != nil {
		t.Fatalf("EnsureDefault org-a (2): %v", err)
	}
	if a1.Id() == "" || a1.Id() != a2.Id() {
		t.Fatalf("EnsureDefault not idempotent: %q vs %q", a1.Id(), a2.Id())
	}

	ctxB := (&organization.Organization{Name: "org-b"}).Namespaced(context.Background())
	b1, err := commercestore.EnsureDefault(datastore.NewNamespaced(ctxB))
	if err != nil {
		t.Fatalf("EnsureDefault org-b: %v", err)
	}
	if b1.Id() == a1.Id() {
		t.Fatalf("CROSS-TENANT: org-b store id == org-a store id (%q)", a1.Id())
	}
}
