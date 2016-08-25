package affiliate

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	writeAffiliateRequired := middleware.TokenRequired(permission.Admin, permission.WriteUser)

	api := rest.New(affiliate.Affiliate{})

	api.GET("/:affiliateid/connect", writeAffiliateRequired, writeAffiliateRequired, connect)
	api.GET("/:affiliateid/callback", writeAffiliateRequired, writeAffiliateRequired, stripeCallback)

	api.Route(router, args...)
}
