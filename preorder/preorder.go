package preorder

import (
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("preorder")

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/order/:token", GetPreorder)
	router.POST("/order/save", SavePreorder)

	router.GET("/thanks", Thanks)
}
