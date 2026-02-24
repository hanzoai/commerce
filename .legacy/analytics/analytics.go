package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/util/router"
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

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start standard HTTP server
	http.ListenAndServe(":"+port, nil)
}
