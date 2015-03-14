package preorder

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("preorder")
	router.Use(middleware.UnavailableHandler())

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/order/:token", GetPreorder)
	router.POST("/order/save", SavePreorder)

	router.GET("/thanks", Thanks)
}
