package auth

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/util/router"
)

func Route(router router.Router, args ...gin.HandlerFunc) {
	router.POST("/auth", credentials)
}
