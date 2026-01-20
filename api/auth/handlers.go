package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	router.POST("/auth", credentials)
}
