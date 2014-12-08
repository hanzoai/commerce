package checkout

import (
	"crowdstart.io/middleware"
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("checkout")

	loginRequired := middleware.LoginRequired("store")
	checkLogin := middleware.CheckLogin()

	router.GET("/", checkLogin, index)
	router.POST("/", checkLogin, checkout)
	router.POST("/charge", loginRequired, charge)
	router.GET("/complete", loginRequired, complete)
}
