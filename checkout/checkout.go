package checkout

import (
	"crowdstart.io/util/router"
)

func init() {
	router := router.New("checkout")

	router.POST("/", checkout)
	router.POST("/authorize", authorize)
	router.GET("/complete", complete)
}
