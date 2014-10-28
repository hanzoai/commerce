package checkout

import (
	"crowdstart.io/util/router"
	"net/http"
)

func init() {
	router := router.New()

	router.GET("/checkout", checkout)
	router.GET("/checkout/", checkout)
	router.GET("/checkout-complete/", checkoutComplete)
	router.POST("/submit-order", submitOrder)

	http.Handle("/", router)
}
