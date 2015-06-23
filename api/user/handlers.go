package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/user"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/rest"
	"crowdstart.com/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	readUserRequired := middleware.TokenRequired(permission.Admin, permission.ReadUser)

	userApi := rest.New(user.User{})
	userApi.Route(router, args...)
	userApi.GET("/:userid/transaction", readUserRequired, getTransactions)
	userApi.PUT("/:userid/transaction", readUserRequired, getTransactions)
}
