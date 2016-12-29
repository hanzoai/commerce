package coupon

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func getCoupon(c *gin.Context) {
	couponid := c.Params.ByName("couponid")

	db := datastore.New(c)
	cpn := coupon.New(db)
	if cpn.Dynamic {
		http.Fail(c, 400, "Failed to get dynamic coupon", nil)
		return
	}

	if err := cpn.GetById(couponid); err != nil {
		http.Fail(c, 404, "Failed to get coupon", err)
		return
	}

	// Check if coupon has been used
	cpn.Enabled = cpn.Redeemable()

	http.Render(c, 200, cpn)
}

func codeFromId(c *gin.Context) {
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

func codeFromList(c *gin.Context) {
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
