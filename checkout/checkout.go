package checkout

import (
	"crowdstart.io/util/router"
	"net/http"
)

func init() {
	router := router.New()

	router.GET("/checkout", checkout)
	router.GET("/checkout/", checkout)
	router.POST("/checkout/submit-order", submitOrder)
	router.GET("/checkout/complete", checkoutComplete)

	http.Handle("/checkout", router)
}
