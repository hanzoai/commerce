package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/middleware"
	"hanzo.io/log"
)

func New(moduleName string) *gin.RouterGroup {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	prefix := strings.TrimSpace(config.Prefixes[moduleName])
	if prefix == "" {
		log.Panic("Unable to determine prefix for module: '%s'", moduleName)
	}

	log.Info("Routing %s to %s", prefix, moduleName)

	router.Use(middleware.Logger())

	// Special error handler for API module returns JSON always
	if moduleName == "api" {
		router.Use(middleware.ErrorHandlerJSON())
	} else {
		router.Use(middleware.ErrorHandler())
	}

	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())
	router.Use(middleware.DetectOverrides())

	if config.IsDevelopment {
		router.Use(middleware.LiveReload())
	}

	http.Handle(prefix, router)

	return router.Group(prefix)
}

func Ok(c *context.Context) {
	c.String(200, "ok\n")
}

func Empty(c *context.Context) {
	c.AbortWithStatus(200)
}

func Robots(c *context.Context) {
	c.String(200, "User-agent: *\nDisallow: /\n")
}
