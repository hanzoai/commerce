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
	readAffiliateRequired := middleware.TokenRequired(permission.Admin, permission.ReadUser)
	writeAffiliateRequired := middleware.TokenRequired(permission.Admin, permission.WriteUser)
	readOrderRequired := middleware.TokenRequired(permission.Admin, permission.ReadOrder)
	readReferralRequired := middleware.TokenRequired(permission.Admin, permission.ReadReferral)
	readReferrerRequired := middleware.TokenRequired(permission.Admin, permission.ReadReferrer)

	api := rest.New(affiliate.Affiliate{})

	api.GET("/:affiliateId/password/reset", writeAffiliateRequired, resetPassword)
	api.GET("/:affiliateId/orders", readAffiliateRequired, readOrderRequired, getOrders)
	api.GET("/:affiliateId/referrals", readAffiliateRequired, readReferralRequired, getReferrals)
	api.GET("/:affiliateId/referrers", readAffiliateRequired, readReferrerRequired, getReferrers)
	api.GET("/:affiliateId/transactions", readAffiliateRequired, getTransactions)

	api.Route(router, args...)
}
