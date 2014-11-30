package checkout

import "crowdstart.io/util/router"

func init() {
	router := router.New("checkout")

	router.GET("/", index)
	router.POST("/", checkout)
	router.POST("/charge", charge)
	router.GET("/complete", complete)
}
