package router

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/middleware"
)

func New() *gin.Engine {
	router := gin.New()

	router.Use(middleware.ErrorHandler())
	router.Use(middleware.NotFoundHandler())
	router.Use(middleware.AddHost())
	router.Use(middleware.AppEngine())
	return router
}
