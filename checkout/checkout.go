package checkout

import (
	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
)

func init() {
	router := router.New("checkout")

	// Middleware
	router.Use(middleware.CheckLogin())

	// Checkout
	router.GET("/", index)
	router.POST("/", checkout)

	// Charge
	router.POST("/charge", charge)

	// Complete
	router.GET("/complete", complete)
}
