package preorder

import (
	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
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
