package checkout

import (
	"crowdstart.io/util/router"
	"net/http"
)

func init() {
	router := router.New()

	router.POST("/checkout/", checkout)
	router.POST("/checkout/authorize", authorize)
	router.GET("/checkout/complete", complete)

	http.Handle("/checkout/", router)
}
