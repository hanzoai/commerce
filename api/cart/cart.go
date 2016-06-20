package cart

import (
	"errors"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/cart"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

type SetReq struct {
	Quantity int    `json:"quantity"`
	Id       string `json:"id"`
	SKU      string `json:"sku"`
	Slug     string `json:"slug"`
}

func Set(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("cartid")

	// Get cart, fail if it doesn't exist
	car := cart.New(db)
	if err := car.Get(id); err != nil {
		http.Fail(c, 404, "No cart found with id: "+id, err)
		return
	}

	// Decode request
	req := SetReq{}
	if err := json.Decode(c.Request.Body, &req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Try to figure out what sort of item we are setting
	var typ string
	if req.Id != "" {
		key, err := hashid.DecodeKey(db.Context, req.Id)
		if err != nil {
			http.Fail(c, 400, "Failed to decode id", err)
			return
		}

		typ = key.Kind()
	} else if req.Slug != "" {
		typ = "product"
	} else if req.SKU != "" {
		typ = "variant"
	} else {
		http.Fail(c, 400, "No product or variant specified", errors.New("No product or variant specified"))
		return
	}

	// Update cart with new item quantity information
	if err := car.SetItem(db, req.Id, typ, req.Quantity); err != nil {
		http.Fail(c, 400, "Failed to update line item", err)
		return
	}

	// Update cart in datastore
	if err := car.Update(); err != nil {
		http.Fail(c, 500, "Failed to update cart", err)
	} else {
		http.Render(c, 200, car)
	}
}

func Discard(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("cartid")

	// Get cart, fail if it doesn't exist
	car := cart.New(db)
	if err := car.Get(id); err != nil {
		http.Fail(c, 404, "No cart found with id: "+id, err)
		return
	}

	car.Status = cart.Discarded

	// Update cart in datastore
	if err := car.Update(); err != nil {
		http.Fail(c, 500, "Failed to update cart", err)
	} else {
		http.Render(c, 200, car)
	}
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	publishedRequired := middleware.TokenRequired(permission.Admin, permission.Published)
	namespaced := middleware.Namespace()

	api := rest.New(cart.Cart{})

	api.POST("/:cartid/set", publishedRequired, namespaced, Set)
	api.POST("/:cartid/discard", publishedRequired, namespaced, Discard)

	api.Route(router, args...)
}
