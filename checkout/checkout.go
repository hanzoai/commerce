package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("checkout")

	// Middleware
	router.Use(middleware.CheckLogin())
	loginRequired := middleware.LoginRequired("store")

	// Checkout
	router.GET("/", index)
	router.POST("/", checkout)

	// Charge
	router.POST("/charge", charge)

	// Complete
	router.GET("/complete", loginRequired, complete)
}
