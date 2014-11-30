package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"strings"

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

	log.Info("Using prefix %s for module %s", prefix, moduleName)

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
