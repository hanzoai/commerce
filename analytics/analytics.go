package main

import (
	"github.com/gin-gonic/gin"
	"google.golang.org/appengine"

	"hanzo.io/middleware"
	"hanzo.io/util/router"
)

func main() {
	analytics := router.New("analytics")

	// Use permissive CORS policy for all API routes.
	analytics.Use(middleware.AccessControl("*"))
	analytics.OPTIONS("*wildcard", func(c *gin.Context) {
		c.Next()
	})

	analytics.POST("/:organizationid", create)

	analytics.GET("/", router.Ok)
	analytics.HEAD("/", router.Empty)

	appengine.Main()
}
