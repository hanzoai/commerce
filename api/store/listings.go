package store

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/models2/store"
	"crowdstart.io/util/json"
)

// Return all listings
func getListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	c.JSON(200, stor.Listings)
}

func createListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Zero out listings for clean create
	stor.Listings = make(store.Listings)

	// Decode response body to create new listings
	if err := json.Decode(c.Request.Body, stor.Listings); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		c.JSON(201, stor.Listings)
	}
}

func patchListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Decode response body to update listings
	if err := json.Decode(c.Request.Body, stor.Listings); err != nil {
		json.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.JSON(200, stor.Listings)
	}
}

func deleteListings(c *gin.Context) {
	storeid := c.Params.ByName("id")

	db := datastore.New(c)

	stor := store.New(db)
	if err := stor.Get(storeid); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", storeid, err), err)
		return
	}

	// Zero out listings for clean create
	stor.Listings = make(store.Listings)

	// Try to save store
	if err := stor.Put(); err != nil {
		json.Fail(c, 500, "Failed to save store listings", err)
	} else {
		c.Data(204, "application/json", make([]byte, 0))
	}
}
