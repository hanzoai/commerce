package router

import (
	"crowdstart.io/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
)

func New(path string) *gin.RouterGroup {
	router := gin.New()

	router.Use(middleware.LiveReload())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())

	http.Handle(path, router)

	return router.Group(path)
}
