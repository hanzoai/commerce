package marketing

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/ads/ad"
	"github.com/hanzoai/commerce/models/ads/adcampaign"
	"github.com/hanzoai/commerce/models/ads/adconfig"
	"github.com/hanzoai/commerce/models/ads/adset"
	"github.com/hanzoai/commerce/util/permission"
	"github.com/hanzoai/commerce/util/rest"
	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)
	namespaced := middleware.Namespace()

	api := router.Group("marketing")
	api.Use(adminRequired)

	api.POST("", adminRequired, namespaced, create)

	rest.New(adcampaign.AdCampaign{}).Route(api)
	rest.New(adconfig.AdConfig{}).Route(api)
	rest.New(adset.AdSet{}).Route(api)
	rest.New(ad.Ad{}).Route(api)
}
