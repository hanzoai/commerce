package user

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/user"
	"hanzo.io/util/permission"
	"hanzo.io/util/rest"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	readUserRequired := middleware.TokenRequired(permission.Admin, permission.ReadUser)
	writeUserRequired := middleware.TokenRequired(permission.Admin, permission.WriteUser)
	readOrderRequired := middleware.TokenRequired(permission.Admin, permission.ReadOrder)
	readReferralRequired := middleware.TokenRequired(permission.Admin, permission.ReadReferral)
	readReferrerRequired := middleware.TokenRequired(permission.Admin, permission.ReadReferrer)

	api := rest.New(user.User{})
	api.GET("/:userid/password/reset", writeUserRequired, resetPassword)
	api.GET("/:userid/orders", readUserRequired, readOrderRequired, getOrders)
	api.GET("/:userid/referrals", readUserRequired, readReferralRequired, getReferrals)
	api.GET("/:userid/referrers", readUserRequired, readReferrerRequired, getReferrers)
	api.GET("/:userid/transactions", readUserRequired, getTransactions)
	api.GET("/:userid/transfer", readUserRequired, getTransfers)
	api.GET("/:userid/affiliate", readUserRequired, getAffiliate)

	api.Route(router, args...)
}
