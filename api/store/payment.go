package store

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.io/api/payment"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/store"
	"crowdstart.io/util/json"
)

func authorize(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	id := c.Params.ByName("storeid")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	// Set store and do authorize
	c.Set("store", stor)
	payment.Authorize(c)
}

func charge(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	id := c.Params.ByName("storeid")

	// Get store
	stor := store.New(db)
	if err := stor.GetById(id); err != nil {
		json.Fail(c, 500, fmt.Sprintf("Failed to retrieve store '%v': %v", id, err), err)
		return
	}

	// Set store and do charge
	c.Set("store", stor)
	payment.Charge(c)
}
