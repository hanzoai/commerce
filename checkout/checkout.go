package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("checkout")
	router.Use(middleware.LoginRequired("checkout"))

	router.GET("/", index)
	router.POST("/", checkout)
	router.POST("/charge", charge)
	router.GET("/complete", complete)
}
