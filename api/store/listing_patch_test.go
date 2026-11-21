// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package store

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	commercestore "github.com/hanzoai/commerce/models/store"
)

// PATCH on a listing applies the body to the stored listing: a plain field it names
// changes, and the change is what the store holds afterwards.
func TestPatchListingAppliesTheBody(t *testing.T) {
	defer withResolver(t)()

	org := &organization.Organization{Name: "karma"}
	db := datastore.NewNamespaced(org.Namespaced(context.Background()))
	stor, err := commercestore.EnsureDefault(db)
	if err != nil {
		t.Fatal(err)
	}
	stor.Listings = map[string]commercestore.Listing{"k": {SKU: "before"}}
	if err := stor.Put(); err != nil {
		t.Fatal(err)
	}

	app := zip.New(zip.Config{DisableStartupMessage: true})
	app.Raw(http.MethodPatch, "/v1/store/:storeid/listing/:key", func(c *zip.Ctx) error {
		c.Locals("organization", org)
		return patchListing(c)
	})
	req := httptest.NewRequest(http.MethodPatch, "/v1/store/"+stor.Id()+"/listing/k", strings.NewReader(`{"sku":"after"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status %d, want 200", resp.StatusCode)
	}

	got := commercestore.New(db)
	if err := got.GetById(stor.Id()); err != nil {
		t.Fatal(err)
	}
	if sku := got.Listings["k"].SKU; sku != "after" {
		t.Fatalf("stored listing sku %q after PATCH, want \"after\"", sku)
	}
}
