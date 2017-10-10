package marketing

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/ads/ad"
	"hanzo.io/models/ads/adcampaign"
	"hanzo.io/models/ads/adconfig"
	"hanzo.io/models/ads/adset"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
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
