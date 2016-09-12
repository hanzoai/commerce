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
	namespaced := middleware.Namespace()

	api := rest.New(affiliate.Affiliate{})
	api.Create = create(api)

	api.GET("/:affiliateid/connect", writeAffiliateRequired, namespaced, connect)
	api.GET("/:affiliateid/callback", writeAffiliateRequired, namespaced, stripeCallback)
	api.GET("/:affiliateid/enable", writeAffiliateRequired, namespaced, stripeCallback)
	api.GET("/:affiliateid/disable", writeAffiliateRequired, namespaced, stripeCallback)

	api.Route(router, args...)
}
