package preorder

import (
	"crowdstart.io/config"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New(config.Get().PrefixFor("preorder"))

	router.GET("/", Index)
	router.POST("/", Login)

	router.GET("/:token", GetPreorder)
	router.POST("/preorder", SavePreorder)
}
