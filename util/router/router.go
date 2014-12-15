package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
)

func New(moduleName string) *gin.RouterGroup {
	router := gin.New()

	prefix := strings.TrimSpace(config.Prefixes[moduleName])
	if prefix == "" {
		log.Panic("Unable to determine prefix for module: '%s'", moduleName)
	}

	log.Info("Routing %s to %s", prefix, moduleName)

	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())

	if config.IsDevelopment {
		router.Use(middleware.LiveReload())
	}

	http.Handle(prefix, router)

	return router.Group(prefix)
}
