package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/log"
)

func New(moduleName string) *gin.RouterGroup {
	router := gin.New()

	prefix := config.Get().PrefixFor(moduleName)
	if prefix == "" {
		log.Panic("Unable to determine prefix for module: '%s'", moduleName)
	}

	router.Use(middleware.ErrorHandler())
	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())

	if config.Get().Development {
		router.Use(middleware.LiveReload())
	}

	http.Handle(prefix, router)

	return router.Group(prefix)
}
