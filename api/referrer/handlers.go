package referrer

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/referrer"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	readReferralRequired := middleware.TokenRequired(permission.Admin, permission.ReadReferral)

	api := rest.New(referrer.Referrer{})
	api.GET("/:referrerid/referrals", readReferralRequired, getReferrals)
	api.GET("/:referrerid/transactions", readReferralRequired, getTransactions)

	api.Route(router, args...)
}
