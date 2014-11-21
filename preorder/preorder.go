package preorder

import (
	"crowdstart.io/config"
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New(config.Get().PrefixFor("preorder"))

	router.GET("/", middleware.LoginRequired(), Preorder)
	router.GET("/:token", WithToken)
}
