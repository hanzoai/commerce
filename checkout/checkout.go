package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

var loginRequired = middleware.LoginRequired("checkout")

func init() {
	router := router.New("checkout")

	router.GET("/", index)
	router.POST("/", checkout)
	router.POST("/charge", loginRequired, charge)
	router.GET("/complete", loginRequired, complete)
}
