package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"crowdstart.io/config"
	"crowdstart.io/middleware"
)

func New(path string) *gin.RouterGroup {
	router := gin.New()

	router.Use(middleware.ErrorHandler())
	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())

	if config.Get().Development {
		router.Use(middleware.LiveReload())
	}

	http.Handle(path, router)

	return router.Group(path)
}
