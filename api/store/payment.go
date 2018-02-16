package store

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/store"
	"hanzo.io/util/json/http"

	checkoutApi "hanzo.io/api/checkout"
)

func setStore(c *context.Context) error {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	id := c.Params.ByName("storeid")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		http.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return err
	}

	// Set store and do authorize
	c.Set("store", stor)
	return nil
}

func authorize(c *context.Context) {
	if err := setStore(c); err == nil {
		checkoutApi.Authorize(c)
	}
}

func capture(c *context.Context) {
	if err := setStore(c); err == nil {
		checkoutApi.Capture(c)
	}
}

func charge(c *context.Context) {
	if err := setStore(c); err == nil {
		checkoutApi.Charge(c)
	}
}

func confirm(c *context.Context) {
	if err := setStore(c); err == nil {
		checkoutApi.Confirm(c)
	}
}

func cancel(c *context.Context) {
	if err := setStore(c); err == nil {
		checkoutApi.Cancel(c)
	}
}
