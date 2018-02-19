package coupon

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/coupon"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/log"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func getCoupon(c *context.Context) {
	couponid := c.Params.ByName("couponid")

	db := datastore.New(c)
	cpn := coupon.New(db)

	if err := cpn.GetById(couponid); err != nil {
		http.Fail(c, 404, "Failed to get coupon", err)
		return
	}

	// if cpn.Dynamic {
	// 	http.Fail(c, 400, "Failed to get dynamic coupon", nil)
	// 	return
	// }

	// Check if coupon has been used
	cpn.Enabled = cpn.Redeemable()

	http.Render(c, 200, cpn)
}

func codeFromId(c *context.Context) {
	couponid := c.Params.ByName("couponid")
	uniqueid := c.Params.ByName("uniqueid")

	db := datastore.New(c)
	cpn := coupon.New(db)
	if err := cpn.GetById(couponid); err != nil {
		http.Fail(c, 404, "Failed to get coupon", err)
		return
	}

	cpn.Code_ = cpn.CodeFromId(uniqueid)

	log.Debug("%#v", cpn)

	// Check if coupon has been used
	cpn.Enabled = cpn.Redeemable()

	http.Render(c, 200, cpn)
}

func codeFromList(c *context.Context) {
	couponid := c.Params.ByName("couponid")

	db := datastore.New(c)
	cpn := coupon.New(db)
	if err := cpn.GetById(couponid); err != nil {
		http.Fail(c, 404, "Failed to get coupon %v", err)
		return
	}

	list := make([]string, 0)

	// Decode response body to create new order
	if err := json.Decode(c.Request.Body, list); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	codes := make([]string, len(list))

	for _, id := range list {
		codes = append(codes, cpn.CodeFromId(id))
	}

	http.Render(c, 200, codes)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := rest.New(coupon.Coupon{})

	api.Get = getCoupon
	api.GET("/:couponid/code/:uniqueid", adminRequired, namespaced, codeFromId)
	api.POST("/:couponid/code", adminRequired, namespaced, codeFromList)

	api.Route(router, args...)
}
