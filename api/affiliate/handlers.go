package affiliate

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/affiliate"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
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
