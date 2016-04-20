package coupon

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/coupon"
	"crowdstart.com/util/hashid"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func getCoupon(c *gin.Context) {
	couponid := c.Params.ByName("couponid")

	ids := make([]int, 0)

	// Assume normal coupon
	coup := coupon.New(c)
	err := coup.GetById(couponid)

	// Try to decode as dynamic coupon
	if err != nil {
		ids = hashid.Decode(couponid)
		err = coup.GetById(ids[0])
	}

	if err != nil {
		http.Fail(c, 404, "Failed to get coupon", err)
		return
	}

	// Check if coupon has been used
	coup.Enabled = coup.Redeemable()

	return coup
}

func codeFromId(c *gin.Context) {
	couponid := c.Params.ByName("couponid")
	uniqueid := c.Params.ByName("uniqueid")

	ctx := middleware.GetAppEngine(c)
	cid, _ := hashid.DecodeKey(ctx, couponid)
	uid, _ := hashid.DecodeKey(ctx, uniqueid)

	code := hashid.Encode(int(cid.IntID()), int(uid.IntID()))

	http.Render(c, 200, code)
}

func codeFromList(c *gin.Context) {
	couponid := c.Params.ByName("couponid")

	ctx := middleware.GetAppEngine(c)
	cid, _ := hashid.DecodeKey(ctx, couponid)

	list := make([]string, 0)

	// Decode response body to create new order
	if err := json.Decode(c.Request.Body, list); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	codes := make([]string, len(list))

	for _, id := range list {
		uid, _ := hashid.DecodeKey(ctx, id)
		codes = append(codes, hashid.Encode(int(cid.IntID()), int(uid.IntID())))
	}

	http.Render(c, 200, codes)
}

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := rest.New(coupon.Coupon{})

	api.Get("/:couponid", adminRequired, namespaced, getCoupon)
	api.GET("/:couponid/code/:uniqueid", adminRequired, namespaced, codeFromId)
	api.POST("/:couponid/code", adminRequired, namespaced, codeFromList)

	api.Route(router, args...)
}
