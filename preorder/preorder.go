package preorder

import (
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("preorder")

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/order/:token", GetMultiPreorder)
	router.POST("/order/save", SaveMultiPreorder)

	router.GET("/thanks", Thanks)
}
