package store

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

// Return all listings
func listListing(c *gin.Context) {
	id := c.Params.ByName("storeid")
	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	http.Render(c, 200, stor.Listings)
}

// Get single store listing for given product/variant
func getListing(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	id := c.Params.ByName("storeid")
	key := c.Params.ByName("key")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
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
		http.Fail(c, 404, msg, errors.New(msg))
		return
	}

	http.Render(c, 200, listing)
}

func createListing(c *gin.Context) {
	id := c.Params.ByName("storeid")
	key := c.Params.ByName("key")
	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		msg := fmt.Sprintf("Failed to retrieve store '%v'", id)
		http.Fail(c, 404, msg, err)
		return
	}

	// Try and grab listing
	listing, ok := stor.Listings[key]

	// Do not override on create, user should explicitly update instead
	if ok {
		msg := fmt.Sprintf("'%v' already exists in store '%v' listing")
		http.Fail(c, 400, msg, errors.New(msg))
		return
	}

	// Decode response body for listing
	if err := json.Decode(c.Request.Body, &listing); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update include listing in listings
	stor.AddListing(key, listing)

	// Save store
	if err := stor.Put(); err != nil {
		http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, stor.Listings)
	}
}

func updateListing(c *gin.Context) {
	id := c.Params.ByName("storeid")
	key := c.Params.ByName("key")
	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	listing, ok := stor.Listings[key]

	// Decode response body to create new listings
	if err := json.Decode(c.Request.Body, &listing); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Override listing potentially
	stor.Listings[key] = listing

	// Try to save store
	if err := stor.Put(); err != nil {
		http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		if ok {
			http.Render(c, 200, stor.Listings)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path)
			http.Render(c, 201, stor.Listings)
		}
	}
}

func patchListing(c *gin.Context) {
	id := c.Params.ByName("storeid")
	key := c.Params.ByName("key")
	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	listing, ok := stor.Listings[key]
	// Can't patch an object that doesn't exist!
	if !ok {
		msg := fmt.Sprintf("No listing exists for '%v' in store '%v'", key, id)
		http.Fail(c, 404, msg, errors.New(msg))
		return
	}

	// Decode response body to update listings
	if err := json.Decode(c.Request.Body, &listing); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		http.Render(c, 200, stor.Listings)
	}
}

func deleteListing(c *gin.Context) {
	id := c.Params.ByName("storeid")
	key := c.Params.ByName("key")
	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	// Check if file even exists
	_, ok := stor.Listings[key]
	if !ok {
		msg := fmt.Sprintf("No listing exists for '%v' in store '%v'", key, id)
		http.Fail(c, 404, msg, errors.New(msg))
		return
	}

	// Delete key from map
	delete(stor.Listings, key)

	// Try to save store
	if err := stor.Put(); err != nil {
		http.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Data(204, "application/json", make([]byte, 0))
	}
}
