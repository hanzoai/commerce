package affiliate

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/affiliate"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	namespaced := middleware.Namespace()
	tokenRequired := middleware.TokenRequired()
	// writeAffiliateRequired := middleware.TokenRequired(permission.Admin, permission.WriteUser)

	api := rest.New(affiliate.Affiliate{})
	api.Create = create(api)

	api.GET("/:affiliateid/connect", tokenRequired, namespaced, connect)

	api.Route(router, args...)
}
