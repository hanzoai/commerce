package store

import (
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/store"
	"github.com/hanzoai/commerce/util/json/http"

	checkoutApi "github.com/hanzoai/commerce/api/checkout"
)

// setStore loads the store into the request locals and returns a non-nil error
// (having ALREADY rendered the failure via http.Fail) when it can't — the
// payment wrappers below gate their checkout delegation on that error.
func setStore(c *zip.Ctx) error {
	ctx := c.Context()
	db := datastore.New(ctx)
	id := c.Param("storeid")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return err
	}

	// Set store and do authorize
	c.Locals("store", stor)
	return nil
}

func authorize(c *zip.Ctx) error {
	if err := setStore(c); err == nil {
		return checkoutApi.Authorize(c)
	}
	return nil
}

func capture(c *zip.Ctx) error {
	if err := setStore(c); err == nil {
		return checkoutApi.Capture(c)
	}
	return nil
}

func charge(c *zip.Ctx) error {
	if err := setStore(c); err == nil {
		return checkoutApi.Charge(c)
	}
	return nil
}

func confirm(c *zip.Ctx) error {
	if err := setStore(c); err == nil {
		return checkoutApi.Confirm(c)
	}
	return nil
}

func cancel(c *zip.Ctx) error {
	if err := setStore(c); err == nil {
		return checkoutApi.Cancel(c)
	}
	return nil
}
