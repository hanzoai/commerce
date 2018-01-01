package transaction

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/permission"
	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	adminRequired := middleware.TokenRequired(permission.Admin)

	api := router.Group("transaction")

	// Auth and Capture Flow (Two-step Payment)
	api.POST("", adminRequired, Create)
	api.GET("/:kind/:id", adminRequired, List)
	api.POST("/hold", adminRequired, CreateHold)
	api.DELETE("/hold/:id", adminRequired, RemoveHold)
}
