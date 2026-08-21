// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package store

// core.go — the two store questions, asked with values instead of a request.
//
// The content storefront reads an org's store and upserts a product listing from
// a process that is not commerce, and it used to do that by re-entering
// commerce's own HTTP door. Asking over the internal plane needs the questions
// answerable WITHOUT a *zip.Ctx, and the alternative is re-deriving them on the
// other side — a second implementation of "which store is this org's", which is
// how two surfaces come to publish into different stores.
//
// The handlers in current.go and listing.go are thin callers of these.

import (
	"context"
	"fmt"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/util/json"
)

// Current is the org's own store, provisioned on first ask.
//
// It resolves inside the org's OWN namespace, never the shared system DB —
// reading the shared one always returned the phantom "default", which the
// storefront edge treats as unconfigured, so it skipped publishing every org's
// listing image. An org with no store yet gets the canonical default created
// (idempotent, org-scoped, no payment credentials), because the alternative is
// handing back an id that resolves to nothing.
func Current(ctx context.Context, org *organization.Organization) (*store.Store, error) {
	if org == nil {
		return nil, fmt.Errorf("store: no organization")
	}
	db := datastore.NewNamespaced(org.Namespaced(ctx))

	var stores []store.Store
	if _, err := store.New(db).Query().All().Limit(1).GetAll(&stores); err == nil && len(stores) > 0 {
		return &stores[0], nil
	}
	s, err := store.EnsureDefault(db)
	if err != nil {
		return nil, fmt.Errorf("store: provision default: %w", err)
	}
	return s, nil
}

// SetListing upserts one product listing on a store, keyed by the product slug.
//
// patch is DECODED ONTO the existing listing rather than replacing it, which is
// what makes this an override and not a wipe: a caller setting a header image
// preserves the curated name, price and copy it says nothing about. That is the
// same decode the HTTP handler performs, and it is the whole contract of the
// route — replacing instead would silently blank a merchandiser's work.
// It reports whether the listing ALREADY existed, because that is the difference
// between a 200 and a 201 on the HTTP door and the caller is the only one that
// can say it — and it hands back the resulting listings, so a caller renders what
// was actually stored rather than what it hoped it sent.
func SetListing(ctx context.Context, org *organization.Organization, storeID, key string, patch []byte) (listings store.Listings, existed bool, err error) {
	if org == nil {
		return nil, false, fmt.Errorf("store: no organization")
	}
	if storeID == "" || key == "" {
		return nil, false, fmt.Errorf("store: listing needs a store and a key")
	}
	db := datastore.NewNamespaced(org.Namespaced(ctx))

	stor := store.New(db)
	if err := stor.GetById(storeID); err != nil {
		return nil, false, fmt.Errorf("store: retrieve %q: %w", storeID, err)
	}
	listing, existed := stor.Listings[key]
	if err := json.DecodeBytes(patch, &listing); err != nil {
		return nil, existed, fmt.Errorf("store: decode listing: %w", err)
	}
	stor.Listings[key] = listing
	if err := stor.Put(); err != nil {
		return nil, existed, fmt.Errorf("store: save listings: %w", err)
	}
	return stor.Listings, existed, nil
}
