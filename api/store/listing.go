package store

import (
	"errors"
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// orgNamespacedDB returns a datastore scoped to the authenticated org's namespace.
// All listing handlers MUST use this instead of raw datastore.New(c) to ensure
// tenant isolation.
func orgNamespacedDB(c *zip.Ctx) *datastore.Datastore {
	org := middleware.GetOrganization(c)
	// Red MED-1: datastore.New binds the shared systemDB regardless of the ctx
	// namespace, so the listing sub-routes (create/update/patch/delete/get/list)
	// read/write systemDB while the store itself is created per-org via
	// NewNamespaced (rest.newEntity) — a mismatch that 404s every listing op on a
	// real per-org store. NewNamespaced routes to the caller org's own SQLite,
	// matching where the store lives.
	return datastore.NewNamespaced(org.Namespaced(c.Context()))
}

// Return all listings
func listListing(c *zip.Ctx) error {
	id := c.Param("storeid")
	db := orgNamespacedDB(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
	}

	return http.Render(c, 200, stor.Listings)
}

// Get single store listing for given product/variant
func getListing(c *zip.Ctx) error {
	db := orgNamespacedDB(c)
	id := c.Param("storeid")
	key := c.Param("key")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
	}

	// Try and grab listing
	listing, ok := stor.Listings[key]

	// Maybe we have a slug or sku?
	if !ok {
		for _, listing = range stor.Listings {
			if key == listing.Slug || key == listing.SKU {
				ok = true
				break
			}
		}
	}

	// Do not override on create, user should explicitly update instead
	if !ok {
		msg := fmt.Sprintf("No listing exists for '%v' in store '%v'", key, id)
		return http.Fail(c, 404, msg, errors.New(msg))
	}

	return http.Render(c, 200, listing)
}

func createListing(c *zip.Ctx) error {
	id := c.Param("storeid")
	key := c.Param("key")
	db := orgNamespacedDB(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		msg := fmt.Sprintf("Failed to retrieve store '%v'", id)
		return http.Fail(c, 404, msg, err)
	}

	// Try and grab listing
	listing, ok := stor.Listings[key]

	// Do not override on create, user should explicitly update instead
	if ok {
		msg := fmt.Sprintf("'%v' already exists in store '%v' listing", key, id)
		return http.Fail(c, 400, msg, errors.New(msg))
	}

	// Decode response body for listing
	if err := json.DecodeBytes(c.Body(), &listing); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Update include listing in listings
	stor.AddListing(key, listing)

	// Save store
	if err := stor.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.SetHeader("Location", c.Path())
		return http.Render(c, 201, stor.Listings)
	}
}

func updateListing(c *zip.Ctx) error {
	id := c.Param("storeid")
	key := c.Param("key")
	db := orgNamespacedDB(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
	}

	listing, ok := stor.Listings[key]

	// Decode response body to create new listings
	if err := json.DecodeBytes(c.Body(), &listing); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Override listing potentially
	stor.Listings[key] = listing

	// Try to save store
	if err := stor.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		if ok {
			return http.Render(c, 200, stor.Listings)
		} else {
			c.SetHeader("Location", c.Path())
			return http.Render(c, 201, stor.Listings)
		}
	}
}

func patchListing(c *zip.Ctx) error {
	id := c.Param("storeid")
	key := c.Param("key")
	db := orgNamespacedDB(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
	}

	listing, ok := stor.Listings[key]
	// Can't patch an object that doesn't exist!
	if !ok {
		msg := fmt.Sprintf("No listing exists for '%v' in store '%v'", key, id)
		return http.Fail(c, 404, msg, errors.New(msg))
	}

	// Decode response body to update listings
	if err := json.DecodeBytes(c.Body(), &listing); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		return http.Render(c, 200, stor.Listings)
	}
}

func deleteListing(c *zip.Ctx) error {
	id := c.Param("storeid")
	key := c.Param("key")
	db := orgNamespacedDB(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		return http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
	}

	// Check if file even exists
	_, ok := stor.Listings[key]
	if !ok {
		msg := fmt.Sprintf("No listing exists for '%v' in store '%v'", key, id)
		return http.Fail(c, 404, msg, errors.New(msg))
	}

	// Delete key from map
	delete(stor.Listings, key)

	// Try to save store
	if err := stor.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.SetHeader("Content-Type", "application/json")
		return c.Bytes(204, make([]byte, 0))
	}
}
